package provider

import (
	"crypto/rsa"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
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
	Homedir string //move this maybe
	Config  *Config
	db      *sql.DB
	mu      sync.Mutex

	PrivateKey *rsa.PrivateKey
	renters    map[string]*RenterInfo

	StorageReserved int64
	StorageUsed     int64

	TotalBlocks    int
	TotalContracts int
}

const (
	// By default, a provider is configured to provide 10 GB of storage to the network.
	DefaultStorageSpace = 10 * 1e9

	// A provider should provide at least this much space.
	MinStorageSpace = 100 * 1e6
)

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
	StorageReserved int64 `json:"storageReserved"`
	StorageUsed     int64 `json:"storageUsed"`
}

// Loads configuration and database
func LoadFromDisk(homedir string) (*Provider, error) {
	provider := &Provider{
		Homedir: homedir,
	}

	config := &Config{}
	err := util.LoadJson(path.Join(homedir, "config.json"), config)
	if err != nil {
		return nil, fmt.Errorf("Failed to load config file. error: %s", err)
	}
	provider.Config = config

	dbPath := path.Join(homedir, "provider.db")
	provider.db, err = provider.SetupDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize DB. error: %s", err)
	}

	err = provider.LoadDBIntoMemory()
	if err != nil {
		return nil, fmt.Errorf("Failed to load provider DB into mem. error: %s", err)
	}

	privKey, err := loadPrivateKey(path.Join(homedir, "providerid"))
	if err != nil {
		return nil, fmt.Errorf("Failed to load providers private key. error: %s", err)
	}

	provider.PrivateKey = privKey
	return provider, nil
}

// Insert, Delete, and Update activity feeds for each interval
func (provider *Provider) addActivity(op string, bytes int64) error {

	err := provider.InsertActivity()
	if err != nil {
		return fmt.Errorf("Error adding new activity to DB: %s", err)
	}
	err = provider.DeleteActivity()
	if err != nil {
		return fmt.Errorf("Error cycling activity in DB: %s", err)
	}

	// TODO: Abstact and handle errors
	if op == "upload" {
		err = provider.UpdateActivity("BlockUploads", 1)
		if err != nil {
			return fmt.Errorf("add upload activity failed. error: %s", err)
		}
		err = provider.UpdateActivity("BytesUploaded", bytes)
		if err != nil {
			return fmt.Errorf("add upload activity failed. error: %s", err)
		}

		provider.TotalBlocks++
		provider.StorageUsed += bytes

	} else if op == "download" {
		err = provider.UpdateActivity("BlockDownloads", 1)
		if err != nil {
			return fmt.Errorf("add download activity failed. error: %s", err)
		}
		err = provider.UpdateActivity("BytesDownloaded", bytes)
		if err != nil {
			return fmt.Errorf("add download activity failed. error:  %s", err)
		}
	} else if op == "delete" {
		provider.UpdateActivity("BlockDeletions", 1)
		if err != nil {
			return fmt.Errorf("add delete activity failed. error:  %s", err)
		}

		provider.TotalBlocks--
		provider.StorageUsed -= bytes

	} else if op == "contract" {
		provider.UpdateActivity("StorageReservations", 1)
		if err != nil {
			return fmt.Errorf("add contract activity failed. error: %s", err)
		}
		provider.StorageReserved += bytes
	}
	return nil
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

func (provider *Provider) getRenterPublicKey(renterId string) (*rsa.PublicKey, error) {
	client := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
	err := client.AuthorizeProvider(provider.PrivateKey, provider.Config.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch authenticate with meta while fetching pubkey. error: %s", err)
	}
	rent, err := client.GetRenter(renterId)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve renter from meta. error: %s", err)
	}
	key, err := util.UnmarshalPublicKey([]byte(rent.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse renters pubkey. error: %s", err)
	}
	return key, nil
}

// Currently called after forming every contract could also be moved into maintenance
// (It will still need to be called in the postInfo method)
func (provider *Provider) UpdateMeta() error {
	pubKeyBytes, err := ioutil.ReadFile(provider.Config.PublicKeyFile)
	if err != nil {
		return fmt.Errorf("Failed to parse pubKey in UpdateMeta. error: %s", err)
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
	resp := &getStatsResp{
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
	return resp
}
