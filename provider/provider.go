package provider

import (
	"crypto/rsa"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"skybin/core"
	"skybin/metaserver"
	"skybin/util"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
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
	// contracts       []*core.Contract
	renters map[string]*RenterInfo
	mu      sync.Mutex
	db      *sql.DB

	StorageReserved int64 `json:"storageReserved"`
	StorageUsed     int64 `json:"storageUsed"`

	TotalBlocks    int `json:"totalBlocks"`
	TotalContracts int `json:"totalContracts`
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
	Timestamps          []string `json:"timestamps"`
	BlockUploads        []int64  `json:"blockUploads"`
	BlockDownloads      []int64  `json:"blockDownloads"`
	BlockDeletions      []int64  `json:"blockDeletions"`
	BytesUploaded       []int64  `json:"bytesUploaded"`
	BytesDownloaded     []int64  `json:"bytesDownloaded"`
	StorageReservations []int64  `json:"storageReservations"`
}
type Recents struct {
	Hour *Summary `json:"hour"`
	Day  *Summary `json:"day"`
	Week *Summary `json:"week"`
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

type BlockInfo struct {
	RenterId string `json:"renterId"`
	BlockId  string `json:"blockId"`
	Size     int64  `json:"blockSize"`
}

type RenterInfo struct {
	// RenterId        string           `json:"RenterId`
	StorageReserved int64            `json:"storageReserved"`
	StorageUsed     int64            `json:"storageUsed"`
	Contracts       []*core.Contract `json:"contracts"`
	Blocks          []*BlockInfo     `json:"blocks"`
}

type snapshot struct {
	// Contracts []*core.Contract       `json:"contracts"`
	// Stats     Stats                  `json:"stats"`
	Renters map[string]*RenterInfo `json:"renters"`
}

func (provider *Provider) saveSnapshot() error {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	s := snapshot{
	// Contracts: provider.contracts,
	// Stats:     provider.stats,
	// Renters: provider.renters,
	}

	return util.SaveJson(path.Join(provider.Homedir, "snapshot.json"), &s)
}

// Loads configuration and snapshot information
func LoadFromDisk(homedir string) (*Provider, error) {
	provider := &Provider{
		Homedir: homedir,
		// contracts: make([]*core.Contract, 0),
		// renters: make(map[string]*RenterInfo, 0),
	}

	config := &Config{}
	err := util.LoadJson(path.Join(homedir, "config.json"), config)
	if err != nil {
		return nil, err
	}
	provider.Config = config

	dbPath := path.Join(homedir, "provider.db")
	provider.db, err = provider.setup_db(dbPath)

	provider.LoadDBIntoMemory()
	// snapshotPath := path.Join(homedir, "snapshot.json")
	// if _, err := os.Stat(snapshotPath); err == nil {
	// 	var s snapshot
	// 	err := util.LoadJson(snapshotPath, &s)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("Unable to load snapshot. Error: %s", err)
	// 	}

	// 	// provider.contracts = s.Contracts
	// 	// provider.stats = s.Stats
	// 	provider.renters = s.Renters
	// }
	// TODO: Recalculate storage reserved and used
	// alternatively: store in snapshot and/or move to maintenance
	// provider.StorageReserved = 0
	// provider.StorageUsed = 0

	//TODO: Use database to calculate this on load
	// for _, r := range provider.renters {
	// 	provider.StorageReserved += r.StorageReserved
	// 	provider.StorageUsed += r.StorageUsed
	// }

	privKey, err := loadPrivateKey(path.Join(homedir, "providerid"))
	if err != nil {
		return nil, err
	}

	provider.PrivateKey = privKey
	return provider, err
}

// Add statistics in Time Series format for charting
func (provider *Provider) addActivity(op string, bytes int64) {
	t := time.Now()
	hour := t.Truncate(time.Minute * 5)
	day := t.Truncate(time.Hour)
	week := t.Truncate(time.Hour * 24)

	// if activity for this interval does not already exist create it
	provider.InsertActivity("hour", hour)
	provider.InsertActivity("day", day)
	provider.InsertActivity("week", week)

	provider.DeleteActivity()

	if op == "upload" {
		provider.UpdateActivity("hour", hour, "BlockUploads", 1)
		provider.UpdateActivity("day", day, "BlockUploads", 1)
		provider.UpdateActivity("week", week, "BlockUploads", 1)

		provider.UpdateActivity("hour", hour, "BytesUploaded", bytes)
		provider.UpdateActivity("day", day, "BytesUploaded", bytes)
		provider.UpdateActivity("week", week, "BytesUploaded", bytes)

		provider.TotalBlocks++
		provider.StorageUsed += bytes
	}
	if op == "download" {
		provider.UpdateActivity("hour", hour, "BlockDownloads", 1)
		provider.UpdateActivity("day", day, "BlockDownloads", 1)
		provider.UpdateActivity("week", week, "BlockDownloads", 1)

		provider.UpdateActivity("hour", hour, "BytesDownloaded", bytes)
		provider.UpdateActivity("day", day, "BytesDownloaded", bytes)
		provider.UpdateActivity("week", week, "BytesDownloaded", bytes)
	}
	if op == "delete" {
		provider.UpdateActivity("hour", hour, "BlockDeletions", 1)
		provider.UpdateActivity("day", day, "BlockDeletions", 1)
		provider.UpdateActivity("week", week, "BlockDeletions", 1)

		provider.TotalBlocks--
		provider.StorageUsed -= bytes
	}
	if op == "contract" {
		provider.UpdateActivity("hour", hour, "StorageReservations", 1)
		provider.UpdateActivity("day", day, "StorageReservations", 1)
		provider.UpdateActivity("week", week, "StorageReservations", 1)

		provider.StorageReserved += bytes
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
		TotalContracts:   provider.TotalContracts,
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
	// provider.makeStats()
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

// Initializes an empty stats response
func (provider *Provider) makeStatsResp() *getStatsResp {
	var timestamps []string
	t := time.Now().Truncate(time.Hour)
	currTime := t.Add(-1 * time.Hour * 24)
	for currTime != t {
		currTime = currTime.Add(time.Hour)
		timestamps = append(timestamps, currTime.Format(time.RFC3339))
	}
	resp := getStatsResp{
		ActivityCounter: &Activity{
			Timestamps:          timestamps,
			BlockUploads:        make([]int64, 24),
			BlockDownloads:      make([]int64, 24),
			BlockDeletions:      make([]int64, 24),
			BytesUploaded:       make([]int64, 24),
			BytesDownloaded:     make([]int64, 24),
			StorageReservations: make([]int64, 24),
		},
		RecentSummary: &Recents{
			Hour: &Summary{
				BlockUploads:        0,
				BlockDownloads:      0,
				BlockDeletions:      0,
				StorageReservations: 0,
			},
			Day: &Summary{
				BlockUploads:        0,
				BlockDownloads:      0,
				BlockDeletions:      0,
				StorageReservations: 0,
			},
			Week: &Summary{
				BlockUploads:        0,
				BlockDownloads:      0,
				BlockDeletions:      0,
				StorageReservations: 0,
			},
		},
	}
	return &resp
}
