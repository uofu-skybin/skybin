package provider

import (
	"crypto/rsa"
	"errors"
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
	ProviderID     string        `json:"providerId"`
	PublicApiAddr  string        `json:"publicApiAddress"`
	MetaAddr       string        `json:"metaServerAddress"`
	LocalApiAddr   string        `json:"localApiAddress"`
	PrivateKeyFile string        `json:"privateKeyFile"`
	PublicKeyFile  string        `json:"publicKeyFile"`
	SpaceAvail     int64         `json:"spaceAvail"`
	StorageRate    int64         `json:"storageRate"`
	MinStorageRate int64         `json:"minStorageRate"`
	MaxStorageRate int64         `json:"maxStorageRate"`
	PricingPolicy  PricingPolicy `json:"pricingPolicy"`
}

type Info struct {
	ProviderId       string `json:"providerId"`
	StorageAllocated int64  `json:"storageAllocated"`
	StorageReserved  int64  `json:"storageReserved"`
	StorageUsed      int64  `json:"storageUsed"`
	StorageFree      int64  `json:"storageFree"`
	StorageRate      int64  `json:"storageRate"`
	MinStorageRate   int64  `json:"minStorageRate"`
	MaxStorageRate   int64  `json:"maxStorageRate"`
	PricingPolicy    string `json:"pricingPolicy"`
	TotalContracts   int    `json:"totalContracts"`
	TotalBlocks      int    `json:"totalBlocks"`
	TotalRenters     int    `json:"totalRenters"`
}

type Provider struct {
	Homedir string //move this maybe
	Config  *Config
	privKey *rsa.PrivateKey
	db      *providerDB

	// Maps renter IDs to renter information
	renters map[string]*renterInfo

	// Maps renter IDs to their public keys
	renterKeys      map[string]string
	StorageReserved int64
	StorageUsed     int64
	TotalBlocks     int
	TotalContracts  int
	logger          *log.Logger
	doneCh          chan struct{}
	mu              sync.RWMutex
}

type blockInfo struct {
	RenterId string `json:"renterId"`
	BlockId  string `json:"blockId"`
	Size     int64  `json:"blockSize"`
}

type renterInfo struct {
	StorageReserved int64 `json:"storageReserved"`
	StorageUsed     int64 `json:"storageUsed"`
}

type PricingPolicy string

const (
	FixedPricingPolicy      PricingPolicy = "fixed"
	PassivePricingPolicy    PricingPolicy = "passive"
	AggressivePricingPolicy PricingPolicy = "aggressive"
)

const (
	// By default, a provider is configured to provide 10 GB of storage to the network.
	DefaultStorageSpace = 10 * 1e9

	// A provider should provide at least this much space.
	MinStorageSpace       = 100 * 1e6
	DefaultPricingPolicy  = PassivePricingPolicy
	DefaultMinStorageRate = 1
	DefaultMaxStorageRate = 100000
	DefaultStorageRate    = DefaultMinStorageRate
	PricingUpdateFreq     = 5 * time.Minute
	renterKeyFile         = "renter_keys.json"
)

// Loads configuration and database
func LoadFromDisk(homedir string) (*Provider, error) {
	provider := &Provider{
		Homedir:    homedir,
		doneCh:     make(chan struct{}),
		logger:     log.New(ioutil.Discard, "", log.LstdFlags),
		renterKeys: map[string]string{},
	}

	config := &Config{}
	err := util.LoadJson(path.Join(homedir, "config.json"), config)
	if err != nil {
		return nil, fmt.Errorf("Failed to load config file. error: %s", err)
	}
	provider.Config = config

	dbPath := path.Join(homedir, "provider.db")
	provider.db, err = setupDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize DB. error: %s", err)
	}

	err = provider.loadInfoFromDB()
	if err != nil {
		return nil, fmt.Errorf("Failed to load provider DB into mem. error: %s", err)
	}

	privKey, err := loadPrivateKey(path.Join(homedir, "providerid"))
	if err != nil {
		return nil, fmt.Errorf("Failed to load providers private key. error: %s", err)
	}
	provider.privKey = privKey

	renterKeyFile := path.Join(homedir, renterKeyFile)
	if _, err := os.Stat(renterKeyFile); err == nil {
		renterKeys := map[string]string{}
		err = util.LoadJson(renterKeyFile, &renterKeys)
		if err != nil {
			return nil, fmt.Errorf("Failed to load renter keys. error: %s", err)
		}
		provider.renterKeys = renterKeys
	}
	return provider, nil
}

//  Loads basic memory objects from db
//  These will be recalculated based on db state at each restart
//  (potentially useful for maintenance also)
// - provider.StorageReserved
// - provider.StorageUsed
// - provider.TotalBlocks
// - provider.TotalContracts
// - provider.renters {
// 	   - StorageUsed
//     - StorageReserved
//   }
func (p *Provider) loadInfoFromDB() error {
	p.StorageReserved = 0
	p.StorageUsed = 0
	p.TotalBlocks = 0
	p.TotalContracts = 0
	p.renters = make(map[string]*renterInfo, 0)

	contracts, err := p.db.GetAllContracts()
	if err != nil {
		// fatal?
		return err
	}
	for _, c := range contracts {
		_, ok := p.renters[c.RenterId]
		if !ok {
			p.renters[c.RenterId] = &renterInfo{}
		}
		p.renters[c.RenterId].StorageReserved += c.StorageSpace
		p.StorageReserved += c.StorageSpace
		p.TotalContracts++
	}
	blocks, err := p.db.GetAllBlocks()
	if err != nil {
		// fatal?
		return err
	}
	for _, b := range blocks {
		_, ok := p.renters[b.RenterId]
		if !ok {
			// TODO: block with no associated contract?
			return nil
		}
		p.renters[b.RenterId].StorageUsed += b.Size
		p.StorageUsed += b.Size
		p.TotalBlocks++
	}
	return nil
}

func (provider *Provider) StartBackgroundThreads() {
	go provider.pricingUpdateThread()
}

func (provider *Provider) StopBackgroundThreads() {
	close(provider.doneCh)
}

func (provider *Provider) SetLogger(logger *log.Logger) {
	provider.logger = logger
}

func (provider *Provider) UpdateConfig(config *Config) error {
	switch config.PricingPolicy {
	case FixedPricingPolicy:
	case PassivePricingPolicy:
	case AggressivePricingPolicy:
	default:
		return errors.New("Unrecognized pricing policy.")
	}
	if config.MinStorageRate > config.MaxStorageRate {
		return errors.New("min storage rate cannot exceed max storage rate")
	}

	provider.mu.Lock()
	provider.Config.SpaceAvail = config.SpaceAvail
	provider.Config.MinStorageRate = config.MinStorageRate
	provider.Config.MaxStorageRate = config.MaxStorageRate
	provider.Config.PublicApiAddr = config.PublicApiAddr
	provider.Config.PricingPolicy = config.PricingPolicy
	if config.PricingPolicy == FixedPricingPolicy {

		// Only update our storage rate to match the given config
		// if our pricing policy is fixed. Otherwise the pricing
		// algorithm should determine the rate.
		provider.Config.StorageRate = config.StorageRate
	}
	provider.mu.Unlock()

	provider.updatePricing()

	err := provider.UpdateMeta()
	if err != nil {
		return fmt.Errorf("Unable to update metaserver. Error: %s", err)
	}
	err = util.SaveJson(path.Join(provider.Homedir, "config.json"), provider.Config)
	if err != nil {
		return fmt.Errorf("Unable to save config update. Error: %s", err)
	}
	return nil
}

func (provider *Provider) GetPublicInfo() *Info {
	provider.mu.RLock()
	defer provider.mu.RUnlock()
	return &Info{
		ProviderId:       provider.Config.ProviderID,
		StorageAllocated: provider.Config.SpaceAvail,
		StorageReserved:  provider.StorageReserved,
		StorageUsed:      provider.StorageUsed,
		StorageFree:      provider.Config.SpaceAvail - provider.StorageReserved,
		StorageRate:      provider.Config.StorageRate,
		MinStorageRate:   provider.Config.MinStorageRate,
		MaxStorageRate:   provider.Config.MaxStorageRate,
		PricingPolicy:    string(provider.Config.PricingPolicy),
		TotalContracts:   provider.TotalContracts,
		TotalRenters:     len(provider.renters),
		TotalBlocks:      provider.TotalBlocks,
	}
}

func (provider *Provider) GetPrivateInfo() (*core.ProviderInfo, error) {
	metaService := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
	err := metaService.AuthorizeProvider(provider.privKey, provider.Config.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("Error authenticating with metaserver: %s", err)
	}

	info, err := metaService.GetProvider(provider.Config.ProviderID)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (provider *Provider) getRenterPublicKey(renterId string) (*rsa.PublicKey, error) {
	provider.mu.RLock()
	keyStr, exists := provider.renterKeys[renterId]
	provider.mu.RUnlock()
	if !exists {
		client := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
		err := client.AuthorizeProvider(provider.privKey, provider.Config.ProviderID)
		if err != nil {
			return nil, fmt.Errorf("Failed to fetch authenticate with meta while fetching pubkey. error: %s", err)
		}
		renter, err := client.GetRenter(renterId)
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve renter from meta. error: %s", err)
		}
		provider.mu.Lock()
		provider.renterKeys[renterId] = renter.PublicKey
		provider.mu.Unlock()
		err = util.SaveJson(path.Join(provider.Homedir, renterKeyFile), provider.renterKeys)
		if err != nil {
			fmt.Println("error saving renter keys: ", err)
		}
		keyStr = renter.PublicKey
	}
	key, err := util.UnmarshalPublicKey([]byte(keyStr))
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

	provider.mu.RLock()
	info := core.ProviderInfo{
		ID:          provider.Config.ProviderID,
		PublicKey:   string(pubKeyBytes),
		Addr:        provider.Config.PublicApiAddr,
		SpaceAvail:  provider.Config.SpaceAvail - provider.StorageReserved,
		StorageRate: provider.Config.StorageRate,
	}
	provider.mu.RUnlock()
	metaService := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
	err = metaService.AuthorizeProvider(provider.privKey, provider.Config.ProviderID)
	if err != nil {
		return fmt.Errorf("Error authenticating with metaserver: %s", err)
	}

	// Retrieve provider's current balance.
	currentInfo, err := metaService.GetProvider(provider.Config.ProviderID)
	if err != nil {
		return fmt.Errorf("Error retrieving provider balance: %s", err)
	}
	info.Balance = currentInfo.Balance

	err = metaService.UpdateProvider(&info)
	if err != nil {
		return fmt.Errorf("Error updating provider: %s", err)
	}
	return nil
}

func (provider *Provider) Withdraw(email string, amount int64) error {
	client := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
	err := client.AuthorizeProvider(provider.privKey, provider.Config.ProviderID)
	if err != nil {
		return err
	}

	err = client.ProviderWithdraw(provider.Config.ProviderID, email, amount)
	if err != nil {
		return err
	}

	return nil
}

func (provider *Provider) ListTransactions() ([]core.Transaction, error) {
	client := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
	err := client.AuthorizeProvider(provider.privKey, provider.Config.ProviderID)
	if err != nil {
		return nil, err
	}

	transactions, err := client.GetProviderTransactions(provider.Config.ProviderID)
	if err != nil {
		return nil, err
	}
	return transactions, nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return util.UnmarshalPrivateKey(data)
}
