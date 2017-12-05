package provider

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"skybin/core"
	"skybin/util"
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
	contracts []core.Contract
	activity  []Activity
}

// TODO: add struct for provider settings and/or place them in this structure
type Info struct {
	// MinRate          string `json:"providerRate"`
	TotalStorage    int64 `json:"providerAllocated"`
	ReservedStorage int64 `json:"providerReserved"`
	FreeStorage     int64 `json:"providerFree"`
	UsedStorage     int64 `json:"providerUsed"`
	TotalContracts  int   `json:"providerContracts"`
}

type Activity struct {
	RequestType string        `json:"requestType,omitempty"`
	BlockId     string        `json:"blockId,omitempty"`
	RenterId    string        `json:"renterId,omitempty"`
	TimeStamp   string        `json:"time,omitempty"`
	Contract    core.Contract `json:"contract,omitempty"`
}

func (p *Provider) Info() (*Info, error) {

	// Default storage size, TODO: make dynamic when settings is implemented
	var total int64 = 10000000000
	// Calculate size of total contracted storage
	var reserved int64 = 0
	for _, contract := range p.contracts {
		reserved += contract.StorageSpace
	}

	// Walk the dir and determine total size
	used, err := DirSize(path.Join(p.Homedir, "blocks"))
	if err != nil {
		return nil, err
	}
	return &Info{
		TotalStorage:    total,
		ReservedStorage: reserved,
		UsedStorage:     used,
		FreeStorage:     total - used,
		TotalContracts:  len(p.contracts),
	}, nil
}

type snapshot struct {
	Contracts []core.Contract `json:"contracts"`
}

func (provider *Provider) saveSnapshot() error {
	s := snapshot{
		Contracts: provider.contracts,
	}
	return util.SaveJson(path.Join(provider.Homedir, "snapshot.json"), &s)
}

func LoadFromDisk(homedir string) (*Provider, error) {
	provider := &Provider{
		Homedir: homedir,
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
	}

	return provider, err
}

// calculate total size of all blocks in the dir
// in the future potentially store this info in a json but for now this is easy
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
