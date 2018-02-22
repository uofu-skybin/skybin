package renter

type Config struct {
	RenterId                    string `json:"renterId"`
	Alias                       string `json:"alias"`
	ApiAddr                     string `json:"apiAddress"`
	MetaAddr                    string `json:"metaServerAddress"`
	PrivateKeyFile              string `json:"privateKeyFile"`
	PublicKeyFile               string `json:"publicKeyFile"`
	MaxContractSize             int64  `json:"maxContractSize"`
	MaxBlockSize                int64  `json:"maxBlockSize"`
	DefaultDataBlocks           int    `json:"defaultDataBlocks"`
	DefaultParityBlocks         int    `json:"defaultParityBlocks"`
	DefaultContractDurationDays int    `json:"defaultContractDurationDays"`
}

const (

	// The minimum size of a storage blob
	kMinBlobSize = 1

	// Minimum contract storage amount
	// A user cannot reserve less storage than this
	kMinContractSize = 1024 * 1024

	// Maximum storage amount of any contract
	kDefaultMaxContractSize = 1024 * 1024 * 1024

	// Maximum size of any file block
	kDefaultMaxBlockSize = kDefaultMaxContractSize

	// Erasure encoding defaults
	kDefaultDataBlocks   = 8
	kDefaultParityBlocks = 4

	// Default contract duration - 6 months
	kDefaultContractDurationDays = 30 * 6
)

func DefaultConfig() *Config {
	return &Config{
		MaxContractSize:             kDefaultMaxContractSize,
		MaxBlockSize:                kDefaultMaxBlockSize,
		DefaultDataBlocks:           kDefaultDataBlocks,
		DefaultParityBlocks:         kDefaultParityBlocks,
		DefaultContractDurationDays: kDefaultContractDurationDays,
	}
}
