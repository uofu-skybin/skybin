package provider

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"skybin/metaserver"
	"skybin/util"
	"sync"
	"time"
)

type Config struct {
	ProviderID     string `json:"providerId"`
	PublicApiAddr  string `json:"publicApiAddress"`
	MetaAddr       string `json:"metaServerAddress"`
	LocalApiAddr   string `json:"localApiAddress"`
	PrivateKeyFile string `json:"privateKeyFile"`
	PublicKeyFile  string `json:"publicKeyFile"`
	SpaceAvail     int64  `json:"spaceAvail"`
	StorageRate    int64  `json:"storageRate"`
}

type Provider struct {
	Config     *Config
	Homedir    string //move this maybe
	PrivateKey *rsa.PrivateKey
	contracts  []*core.Contract
	stats      Stats
	renters    map[string]*RenterInfo
	mu         sync.Mutex

	StorageReserved int64 `json:"storageReserved"`
	StorageUsed     int64 `json:"storageUsed"`
	TotalBlocks     int   `json:"totalBlocks"`
}

const (
	// By default, a provider is configured to provide 10 GB of storage to the network.
	DefaultStorageSpace = 10 * 1e9

	// A provider should provide at least this much space.
	MinStorageSpace = 100 * 1e6
)

// Provider node statistics
type Stats struct {
	Hour Activity `json:"hour"`
	Day  Activity `json:"day"`
	Week Activity `json:"week"`
}

// Structure to cycle activity over a set interval
type Activity struct {
	// time interval to truncate time on
	Interval time.Duration `json:"interval"`
	// number of activity cycles to save
	Cycles int `json:"cycles"`

	Timestamps []time.Time `json:"timestamps"`

	BlockUploads   []int64 `json:"blockUploads"`
	BlockDownloads []int64 `json:"blockDownloads"`
	BlockDeletions []int64 `json:"blockDeletions"`

	BytesUploaded   []int64 `json:"bytesUploaded"`
	BytesDownloaded []int64 `json:"bytesDownloaded"`

	StorageReservations []int64 `json:"storageReservations"`
}

type Recents struct {
	Hour *Summary `json:"hour"`
	Day  *Summary `json:"day"`
	Week *Summary `json:"week"`
	// Period string `json:"period"`
	// Counters RecentCounters `json:"counters"`
}
type Summary struct {
	BlockUploads        int64 `json:"blockUploads"`
	BlockDownloads      int64 `json:"blockDownloads"`
	BlockDeletions      int64 `json:"blockDeletions"`
	StorageReservations int64 `json:"storageReservations"`
}
type getStatsResp struct {
	RecentSummary   *Recents  `json:"recentSummary"`
	ActivityCounter *Activity `json:"activityCounters"`
}

func (provider *Provider) makeStatsResp() *getStatsResp {
	resp := getStatsResp{
		ActivityCounter: &provider.stats.Day,
		RecentSummary: &Recents{
			Hour: &Summary{
				BlockUploads:        sum(provider.stats.Hour.BlockUploads),
				BlockDownloads:      sum(provider.stats.Hour.BlockDownloads),
				BlockDeletions:      sum(provider.stats.Hour.BlockDeletions),
				StorageReservations: sum(provider.stats.Hour.StorageReservations),
			},
			Day: &Summary{
				BlockUploads:        sum(provider.stats.Day.BlockUploads),
				BlockDownloads:      sum(provider.stats.Day.BlockDownloads),
				BlockDeletions:      sum(provider.stats.Day.BlockDeletions),
				StorageReservations: sum(provider.stats.Day.StorageReservations),
			},
			Week: &Summary{
				BlockUploads:        sum(provider.stats.Week.BlockUploads),
				BlockDownloads:      sum(provider.stats.Week.BlockDownloads),
				BlockDeletions:      sum(provider.stats.Week.BlockDeletions),
				StorageReservations: sum(provider.stats.Week.StorageReservations),
			},
		},
	}
	return &resp
}

func sum(arr []int64) int64 {
	tot := int64(0)
	for _, i := range arr {
		tot += i
	}
	return tot
}

type BlockInfo struct {
	BlockId string `json:"blockId"`
	Size    int64  `json:"blockSize"`
}
type RenterInfo struct {
	StorageReserved int64            `json:"storageReserved"`
	StorageUsed     int64            `json:"storageUsed"`
	Contracts       []*core.Contract `json:"contracts"`
	Blocks          []*BlockInfo     `json:"blocks"`
}

type snapshot struct {
	Contracts []*core.Contract       `json:"contracts"`
	Stats     Stats                  `json:"stats"`
	Renters   map[string]*RenterInfo `json:"renters"`
}

func (provider *Provider) saveSnapshot() error {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	s := snapshot{
		Contracts: provider.contracts,
		Stats:     provider.stats,
		Renters:   provider.renters,
	}
	err := provider.UpdateMeta()
	if err != nil {
		return fmt.Errorf("Error updating metaserver: %s", err)
	}
	return util.SaveJson(path.Join(provider.Homedir, "snapshot.json"), &s)
}

// Loads configuration and snapshot information
func LoadFromDisk(homedir string) (*Provider, error) {
	provider := &Provider{
		Homedir:   homedir,
		contracts: make([]*core.Contract, 0),
		renters:   make(map[string]*RenterInfo, 0),
	}

	config := &Config{}
	err := util.LoadJson(path.Join(homedir, "config.json"), config)
	if err != nil {
		return nil, err
	}
	provider.Config = config

	snapshotPath := path.Join(homedir, "snapshot.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		var s snapshot
		err := util.LoadJson(snapshotPath, &s)
		if err != nil {
			return nil, fmt.Errorf("Unable to load snapshot. Error: %s", err)
		}

		provider.contracts = s.Contracts
		provider.stats = s.Stats
		provider.renters = s.Renters
	}
	// Recalculate storage reserved and used
	// alternatively: store in snapshot and/or move to maintenance
	// provider.StorageReserved = 0
	// provider.StorageUsed = 0
	for _, r := range provider.renters {
		provider.StorageReserved += r.StorageReserved
		provider.StorageUsed += r.StorageUsed
	}

	// 12 - 5 minute intervals
	provider.stats.Hour.Interval = time.Minute * 5
	provider.stats.Hour.Cycles = 12
	// 24 - 1 hour intervals
	provider.stats.Day.Interval = time.Hour
	provider.stats.Day.Cycles = 24
	// 7 - 1 day intervals
	provider.stats.Week.Interval = time.Hour * 24
	provider.stats.Week.Cycles = 7

	privKey, err := loadPrivateKey(path.Join(homedir, "providerid"))
	if err != nil {
		return nil, err
	}
	provider.PrivateKey = privKey
	return provider, err
}

// Add statistics in Time Series format for charting
func (provider *Provider) addActivity(op string, bytes int64) {
	metrics := []*Activity{&provider.stats.Hour, &provider.stats.Day, &provider.stats.Week}
	for _, act := range metrics {
		// add and cycle activity within metric as needed
		act.cycleActivity()

		// update data in current frame
		idx := len(act.Timestamps) - 1
		if op == "upload" {
			act.BlockUploads[idx]++
			act.BytesUploaded[idx] += bytes
		}
		if op == "download" {
			act.BlockDownloads[idx]++
			act.BytesDownloaded[idx] += bytes
		}
		if op == "delete" {
			act.BlockDeletions[idx]++
		}
		if op == "contract" {
			act.StorageReservations[idx]++
		}
	}
	// update provider information
	if op == "upload" {
		provider.TotalBlocks++
		provider.StorageUsed += bytes
	}
	if op == "delete" {
		provider.TotalBlocks--
		provider.StorageUsed -= bytes
	}
	if op == "contract" {
		provider.StorageReserved += bytes
	}

}

// takes in a pointer to an activity cycle and adds any new activity cycles
// and discards old cycles
func (act *Activity) cycleActivity() {

	// Round current time to nearest time increment
	t := time.Now()
	t = t.Truncate(act.Interval)

	var currTime time.Time
	// if no timestamps set to oldest interval within the frame
	if len(act.Timestamps) == 0 {
		currTime = t.Add(-1 * act.Interval * time.Duration(act.Cycles))
	} else {
		currTime = act.Timestamps[len(act.Timestamps)-1]
	}
	// Fill in empty intervals from most recent frame with 0's
	for currTime != t {
		currTime = currTime.Add(act.Interval)
		act.Timestamps = append(act.Timestamps, currTime)

		act.BlockUploads = append(act.BlockUploads, 0)
		act.BlockDownloads = append(act.BlockDownloads, 0)
		act.BlockDeletions = append(act.BlockDeletions, 0)

		act.BytesUploaded = append(act.BytesUploaded, 0)
		act.BytesDownloaded = append(act.BytesDownloaded, 0)

		act.StorageReservations = append(act.StorageReservations, 0)
	}

	// Discard any out of frame cycles
	if len(act.Timestamps) > act.Cycles {
		idx := len(act.Timestamps) - act.Cycles
		act.Timestamps = act.Timestamps[idx:]

		act.BlockUploads = act.BlockUploads[idx:]
		act.BlockDownloads = act.BlockDownloads[idx:]
		act.BlockDeletions = act.BlockDeletions[idx:]

		act.BytesUploaded = act.BytesUploaded[idx:]
		act.BytesDownloaded = act.BytesDownloaded[idx:]

		act.StorageReservations = act.StorageReservations[idx:]
	}
}

type Info struct {
	ProviderId       string `json:"providerId"`
	StorageAllocated int64  `json:"storageAllocated"`
	StorageReserved  int64  `json:"storageReserved"`
	StorageUsed      int64  `json:"storageUsed"`
	StorageFree      int64  `json:"storageFree"`
	TotalContracts   int    `json:"totalContracts"`
	TotalBlocks      int    `json:"totalBlocks"`
	TotalRenters     int    `json:"totalRenters"`
}

func (provider *Provider) GetInfo() *Info {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return &Info{
		ProviderId:       provider.Config.ProviderID,
		StorageAllocated: provider.Config.SpaceAvail,
		StorageReserved:  provider.StorageReserved,
		StorageUsed:      provider.StorageUsed,
		StorageFree:      provider.Config.SpaceAvail - provider.StorageReserved,
		TotalContracts:   len(provider.contracts),
		TotalRenters:     len(provider.renters),
		TotalBlocks:      provider.TotalBlocks,
	}
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return util.UnmarshalPrivateKey(data)
}

// TODO: this needs an open public key endpoint
func (provider *Provider) getRenterPublicKey(renterId string) (*rsa.PublicKey, error) {
	client := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
	err := client.AuthorizeProvider(provider.PrivateKey, provider.Config.ProviderID)
	if err != nil {
		return nil, err
	}
	rent, err := client.GetRenter(renterId)
	if err != nil {
		return nil, err
	}
	key, err := util.UnmarshalPublicKey([]byte(rent.PublicKey))
	if err != nil {
		return nil, err
	}
	return key, nil
}

// Currently called after forming every contract could also be moved into maintenance
// (It will still need to be called in the postInfo method)
func (provider *Provider) UpdateMeta() error {
	pubKeyBytes, err := ioutil.ReadFile(provider.Config.PublicKeyFile)
	if err != nil {
		log.Fatal("Could not read public key file. Error: ", err)
	}
	info := core.ProviderInfo{
		ID:          provider.Config.ProviderID,
		PublicKey:   string(pubKeyBytes),
		Addr:        provider.Config.PublicApiAddr,
		SpaceAvail:  provider.Config.SpaceAvail - provider.StorageReserved,
		StorageRate: provider.Config.StorageRate,
	}
	metaService := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
	err = metaService.AuthorizeProvider(provider.PrivateKey, provider.Config.ProviderID)
	if err != nil {
		return fmt.Errorf("Error authenticating with metaserver: %s", err)
	}
	err = metaService.UpdateProvider(&info)
	if err != nil {
		return fmt.Errorf("Error updating provider: %s", err)
	}
	return nil
}
