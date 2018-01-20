package provider

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"skybin/core"
	"skybin/util"
	"time"
)

type Config struct {
	ProviderID   string `json:"providerId"`
	Addr         string `json:"address"`
	MetaAddr     string `json:"metaServerAddress"`
	IdentityFile string `json:"identityFile"`
}

type Provider struct {
	Config    *Config
	Homedir   string
	contracts []*core.Contract
	stats     Stats
	activity  []Activity
	renters   map[string]renterStats
}

// Provider node statistics
type Stats struct {
	StorageReserved int64 `json:"storageReserved"`
	StorageUsed     int64 `json:"storageUsed"`
}

// Mirrors Stats currently, probably want some other information
type renterStats struct {
	StorageReserved int64            `json:"storageReserved"`
	StorageUsed     int64            `json:"storageUsed"`
	contracts       []*core.Contract `json:"contracts"`
	blocks          *core.Block      `json:"blocks"`
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
	Renters   map[string]renterStats `json:"renters"`
}

func (provider *Provider) saveSnapshot() error {
	s := snapshot{
		Contracts: provider.contracts,
		Stats:     provider.stats,
		Renters:   provider.renters,
	}
	return util.SaveJson(path.Join(provider.Homedir, "snapshot.json"), &s)
}

// func (provider *Provider) saveConfig() error {
// 	s := snapshot{
// 		Contracts: provider.contracts,
// 		Stats:     provider.stats,
// 		Renters:   provider.renters,
// 	}
// 	return util.SaveJson(path.Join(provider.Homedir, "config.json"), &provider.Config)
// }

// Loads configuration and snapshot information
func LoadFromDisk(homedir string) (*Provider, error) {
	provider := &Provider{
		Homedir:   homedir,
		contracts: make([]*core.Contract, 0),
		renters:   make(map[string]renterStats, 0),
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

	return provider, err
}

func (provider *Provider) addActivity(activity Activity) {
	provider.activity = append(provider.activity, activity)
	if len(provider.activity) > maxActivity {

		// Drop the oldest activity.
		// O(N) but fine for small feed.
		provider.activity = provider.activity[1:]
	}
}

func (provider *Provider) negotiateContract(contract *core.Contract) (*core.Contract, error) {

	// TODO determine if contract is amiable for provider here

	// Sign contract
	contract.ProviderSignature = "signature"
	renterID := "test" // contract.RenterId
	// Add storage space to the renter
	renter := provider.renters[renterID]
	renter.StorageReserved += contract.StorageSpace
	provider.renters[renterID] = renter

	// Record contract and update stats
	provider.contracts = append(provider.contracts, contract)
	provider.stats.StorageReserved += contract.StorageSpace

	// Create a new directory for the renters blocks
	os.MkdirAll(path.Join(provider.Homedir, "blocks", renterID), 0700)

	activity := Activity{
		RequestType: negotiateType,
		Contract:    contract,
		TimeStamp:   time.Now(),
		RenterId:    contract.RenterId,
	}
	provider.addActivity(activity)

	provider.saveSnapshot()
	// if err != nil {
	// 	//TODO log internal server error
	// 	// server.logger.Println("Unable to save snapshot. Error:", err)
	// }
	return contract, nil
}

// func (provider *Provider) removeBlock(renterID string, blockID string) error {

// 	// TODO: we need to get the renter id and authenticate here first

// 	path := path.Join(provider.Homedir, "blocks", renterID, blockID)
// 	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
// 		msg := fmt.Sprintf("Cannot find block with ID %s", blockID)
// 		return fmt.Errorf(msg, err)
// 	}

// 	err := os.Remove(path)
// 	if err != nil {
// 		msg := fmt.Sprintf("Error deleting block %s: %s", blockID, err)
// 		return fmt.Errorf(msg)
// 	}

// 	activity := Activity{
// 		RequestType: deleteBlockType,
// 		BlockId:     blockID,
// 		TimeStamp:   time.Now(),
// 		// RenterId:    params.RenterID,
// 	}
// 	provider.addActivity(activity)

// 	return nil
// }

// helper that could be useful for future auditing
func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
