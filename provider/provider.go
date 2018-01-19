package provider

import (
	"fmt"
	"os"
	"path"
	"skybin/core"
	"skybin/util"
)

type Config struct {
	ProviderID   string `json:"providerId"`
	ApiAddr      string `json:"apiAddress"`
	MetaAddr     string `json:"metaServerAddress"`
	IdentityFile string `json:"identityFile"`
	IsRegistered bool   `json:"isRegistered"`
}

// Provider node statistics
type Stats struct {
	StorageReserved int64 `json:"storageReserved"`
	StorageUsed     int64 `json:"storageUsed"`
}

type Provider struct {
	Config    *Config
	Homedir   string
	contracts []*core.Contract
	stats     Stats
}

type snapshot struct {
	Contracts []*core.Contract `json:"contracts"`
	Stats     Stats            `json:"stats"`
}

func (provider *Provider) saveSnapshot() error {
	s := snapshot{
		Contracts: provider.contracts,
		Stats:     provider.stats,
	}
	return util.SaveJson(path.Join(provider.Homedir, "snapshot.json"), &s)
}

func LoadFromDisk(homedir string) (*Provider, error) {
	provider := &Provider{
		Homedir:   homedir,
		contracts: make([]*core.Contract, 0),
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
	}

	return provider, err
}
