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
	activity   []Activity
	renters    map[string]*RenterInfo
	mu         sync.Mutex
}

const (
	// By default, a provider is configured to provide 10 GB of storage to the network.
	DefaultStorageSpace = 10 * 1e9

	// A provider should provide at least this much space.
	MinStorageSpace = 100 * 1e6
)

// Provider node statistics
// TODO: revisit the naming scheme
type Stats struct {
	StorageReserved int64 `json:"storageReserved"`
	StorageUsed     int64 `json:"storageUsed"`

	// This structure is somewhat messy as it is designed to be plug and play
	// with the chart.js data model
	TimeStamps     []time.Time `json:"time"`
	UploadBytes    []int64     `json:"uploadBytes"`
	UploadBlocks   []int64     `json:"uploadBlocks"`
	DownloadBytes  []int64     `json:"downloadBytes"`
	DownloadBlocks []int64     `json:"downloadBlocks"`
	DeleteBytes    []int64     `json:"deleteBytes"`
	DeleteBlocks   []int64     `json:"deleteBlocks"`
	ContractCount  []int64     `json:"contractCount"`
	ContractSize   []int64     `json:"contractSize"`
}

// type TimeSeries struct {
// 	Time time.Time `json:"time"`
// 	Data int64     `json:"data"`
// }

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

type Activity struct {
	RequestType string         `json:"requestType,omitempty"`
	BlockId     string         `json:"blockId,omitempty"`
	RenterId    string         `json:"renterId,omitempty"`
	TimeStamp   time.Time      `json:"time,omitempty"`
	Contract    *core.Contract `json:"contract,omitempty"`
}

const (
	// Max activity feed size
	maxActivity = 10
)

const (
	// Activity types
	negotiateType   = "NEGOTIATE CONTRACT"
	postBlockType   = "POST BLOCK"
	getBlockType    = "GET BLOCK"
	deleteBlockType = "DELETE BLOCK"
)

type snapshot struct {
	Contracts []*core.Contract       `json:"contracts"`
	Stats     Stats                  `json:"stats"`
	Renters   map[string]*RenterInfo `json:"renters"`
}

func (provider *Provider) saveSnapshot() error {
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

	privKey, err := loadPrivateKey(path.Join(homedir, "providerid"))
	if err != nil {
		return nil, err
	}
	provider.PrivateKey = privKey
	return provider, err
}

func (provider *Provider) addActivity(activity Activity) {
	provider.activity = append(provider.activity, activity)
	if len(provider.activity) > maxActivity {
		// Drop the oldest activity.
		provider.activity = provider.activity[1:]
	}
}

// Add statistics in Time Series format for charting
func (provider *Provider) addStat(op string, bytes int64) {
	// Time increment to distinguish stats (intentionally short for dev purposes)
	d := 5 * time.Minute

	// Round current time to nearest time increment
	t := time.Now()
	t = t.Truncate(d)

	// This could potentially be moved to loadFromDisk
	numStats := len(provider.stats.TimeStamps)
	if numStats == 0 {
		provider.stats.TimeStamps = append(provider.stats.TimeStamps, t)
		provider.stats.UploadBlocks = append(provider.stats.UploadBlocks, int64(0))
		provider.stats.UploadBytes = append(provider.stats.UploadBytes, int64(0))
		provider.stats.DownloadBlocks = append(provider.stats.DownloadBlocks, int64(0))
		provider.stats.DownloadBytes = append(provider.stats.DownloadBytes, int64(0))
		provider.stats.DeleteBlocks = append(provider.stats.DeleteBlocks, int64(0))
		provider.stats.DeleteBytes = append(provider.stats.DeleteBytes, int64(0))
		provider.stats.ContractCount = append(provider.stats.ContractCount, int64(0))
		provider.stats.ContractSize = append(provider.stats.ContractSize, int64(0))
		numStats++
	}

	currTime := provider.stats.TimeStamps[numStats-1]
	if currTime != t {
		// Populate empty timeframes if last timestamp is not current
		for currTime != t {
			currTime = currTime.Add(d)
			provider.stats.TimeStamps = append(provider.stats.TimeStamps, currTime)
			provider.stats.UploadBlocks = append(provider.stats.UploadBlocks, int64(0))
			provider.stats.UploadBytes = append(provider.stats.UploadBytes, int64(0))
			provider.stats.DownloadBlocks = append(provider.stats.DownloadBlocks, int64(0))
			provider.stats.DownloadBytes = append(provider.stats.DownloadBytes, int64(0))
			provider.stats.DeleteBlocks = append(provider.stats.DeleteBlocks, int64(0))
			provider.stats.DeleteBytes = append(provider.stats.DeleteBytes, int64(0))
			provider.stats.ContractCount = append(provider.stats.ContractCount, int64(0))
			provider.stats.ContractSize = append(provider.stats.ContractSize, int64(0))
			numStats++
		}
	}
	if op == "upload" {
		provider.stats.UploadBlocks[numStats-1]++
		provider.stats.UploadBytes[numStats-1] += bytes
	}
	if op == "download" {
		provider.stats.DownloadBlocks[numStats-1]++
		provider.stats.DownloadBytes[numStats-1] += bytes
	}
	if op == "delete" {
		provider.stats.DeleteBlocks[numStats-1]++
		provider.stats.DeleteBytes[numStats-1] += bytes
	}
	if op == "contract" {
		provider.stats.ContractCount[numStats-1]++
		provider.stats.ContractSize[numStats-1] += bytes
	}

	// Drop any activity older than a day
	statCount := int(time.Hour * 24 / d)
	if len(provider.stats.TimeStamps) > statCount {
		idx := len(provider.stats.TimeStamps) - statCount
		provider.stats.TimeStamps = provider.stats.TimeStamps[idx:]
		provider.stats.UploadBlocks = provider.stats.UploadBlocks[idx:]
		provider.stats.UploadBytes = provider.stats.UploadBytes[idx:]
		provider.stats.DownloadBlocks = provider.stats.DownloadBlocks[idx:]
		provider.stats.DownloadBytes = provider.stats.DownloadBytes[idx:]
		provider.stats.DeleteBlocks = provider.stats.DeleteBlocks[idx:]
		provider.stats.DeleteBytes = provider.stats.DeleteBytes[idx:]
		provider.stats.ContractCount = provider.stats.ContractCount[idx:]
		provider.stats.ContractSize = provider.stats.ContractSize[idx:]
	}
}

type Info struct {
	ProviderId       string `json:"providerId"`
	StorageAllocated int64  `json:"storageAllocated"`
	StorageReserved  int64  `json:"storageReserved"`
	StorageUsed      int64  `json:"storageUsed"`
	StorageFree      int64  `json:"storageFree"`
	TotalContracts   int    `json:"totalContracts"`
}

func (provider *Provider) GetInfo() *Info {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return &Info{
		ProviderId:       provider.Config.ProviderID,
		StorageAllocated: provider.Config.SpaceAvail,
		StorageReserved:  provider.stats.StorageReserved,
		StorageUsed:      provider.stats.StorageUsed,
		StorageFree:      provider.Config.SpaceAvail - provider.stats.StorageUsed,
		TotalContracts:   len(provider.contracts),
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
		SpaceAvail:  provider.Config.SpaceAvail - provider.stats.StorageReserved,
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
