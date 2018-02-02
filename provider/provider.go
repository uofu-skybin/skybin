package provider

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"skybin/metaserver"
	"skybin/util"
	"time"
)

type Config struct {
	ProviderID     string `json:"providerId"`
	ApiAddr        string `json:"apiAddress"`
	MetaAddr       string `json:"metaServerAddress"`
	PrivateKeyFile string `json:"privateKeyFile"`
	PublicKeyFile  string `json:"publicKeyFile"`

	SpaceAvail  int64 `json:"spaceAvail"`
	StorageRate int64 `json:"storageRate"`
	// StorageRates []core.StorageRate `json:"storageRates,omitempty"`

	// Is this provider registered with metaservice?
	IsRegistered bool `json:"isRegistered"`
}

type Provider struct {
	Config     *Config
	Homedir    string
	PrivateKey *rsa.PrivateKey
	contracts  []*core.Contract
	stats      Stats
	activity   []Activity
	renters    map[string]*RenterInfo
}

// Provider node statistics
type Stats struct {
	StorageReserved int64 `json:"storageReserved"`
	StorageUsed     int64 `json:"storageUsed"`
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

func (provider *Provider) negotiateContract(contract *core.Contract) (*core.Contract, error) {
	renterKey, err := provider.getRenterPublicKey(contract.RenterId)
	if err != nil {
		return nil, fmt.Errorf("Metadata server does not have an associated renter ID")
	}

	// Verify renters signature
	err = core.VerifyContractSignature(contract, contract.RenterSignature, *renterKey)
	if err != nil {
		return nil, fmt.Errorf("Invalid Renter signature: %s", err)
	}

	// TODO determine if contract is amiable for provider here
	// avail := provider.Config.TotalStorage - provider.stats.StorageReserved
	avail := int64(9999999999)
	if contract.StorageSpace > avail {
		return nil, fmt.Errorf("Provider does not have sufficient storage available")
	}

	// Sign contract
	provSig, err := core.SignContract(contract, provider.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("Error signing contract")
	}
	contract.ProviderSignature = provSig

	// Add storage space to the renter
	renter, exists := provider.renters[contract.RenterId]
	if !exists {
		renter = &RenterInfo{
			Contracts: []*core.Contract{},
			Blocks:    []*BlockInfo{},
		}
		provider.renters[contract.RenterId] = renter
	}
	renter.StorageReserved += contract.StorageSpace
	renter.Contracts = append(renter.Contracts, contract)

	provider.stats.StorageReserved += contract.StorageSpace
	provider.contracts = append(provider.contracts, contract)

	activity := Activity{
		RequestType: negotiateType,
		Contract:    contract,
		TimeStamp:   time.Now(),
		RenterId:    contract.RenterId,
	}
	provider.addActivity(activity)

	err = provider.saveSnapshot()
	if err != nil {

		// TODO: Remove contract. I don't do this here
		// since we need to move to an improved storage scheme anyways.
		return nil, fmt.Errorf("Unable to save contract. Error: %s", err)
	}
	return contract, nil
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
	client.AuthorizeProvider(provider.PrivateKey, provider.Config.ProviderID)
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

// THis method will clean up expired files and confirm that they were
// paid for any storage they used
func (provider *Provider) maintanence() {
	// check payments
	// clean up old contracts
	// delete unpaid files
}
