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
	Addr         string `json:"address"`
	MetaAddr     string `json:"metaServerAddress"`
	IdentityFile string `json:"identityFile"`
	//TODO: potentially move to secondary structure
	ProviderRate  string `json:"providerRate"`
	ProviderTotal string `json:"providerTotal"`
	ProviderAvail string `json:"providerAvail"`
}

type Provider struct {
	Config    *Config
	Homedir   string
	contracts []core.Contract
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
