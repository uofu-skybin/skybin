package core

import (
	"time"
)

const (
	DefaultMetaAddr           = "127.0.0.1:8001"
	DefaultRenterAddr         = "127.0.0.1:8002"
	DefaultPublicProviderAddr = ":8003"
	DefaultLocalProviderAddr  = "127.0.0.1:8004"
)

type ProviderInfo struct {
	ID          string `json:"id,omitempty"`
	PublicKey   string `json:"publicKey"`
	Addr        string `json:"address"`
	SpaceAvail  int64  `json:"spaceAvail,omitempty"`
	// Rate charged for storage, in tenths-of-cents/gb/month
	// where 1 gb is 1e9 bytes
	StorageRate int64  `json:"storageRate"`
	// The provider's balance, in tenths of cents.
	Balance int64 `json:"balance"`
}

type RenterInfo struct {
	ID        string   `json:"id"`
	Alias     string   `json:"alias"`
	PublicKey string   `json:"publicKey"`
	Files     []string `json:"files"`
	Shared    []string `json:"shared"`
	// The renter's balance, in tenths of cents.
	Balance int64 `json:"balance"`
}

type Contract struct {
	ID                string    `json:"contractId"`
	RenterId          string    `json:"renterId"`
	ProviderId        string    `json:"providerId"`
	StorageSpace      int64     `json:"storageSpace"`
	StorageFee        int64     `json:"storageFee"`
	StartDate         time.Time `json:"startDate"`
	EndDate           time.Time `json:"endDate"`
	RenterSignature   string    `json:"renterSignature"`
	ProviderSignature string    `json:"providerSignature"`
}

// PaymentInfo contains the details describing payment data for a given
// contract.
type PaymentInfo struct {
	// The contract that this payment information is associated with.
	ContractID string `json:"contract"`
	// Whether or not the renter is currently "paying down" the contract.
	IsPaying bool `json:"isPaying"`
	// The date of the last payment.
	LastPaymentTime time.Time `json:"lastPaymentTime"`
	// The remaining amount to be paid on the contract.
	Balance int64 `json:"balance"`
}

// Transaction describes a transaction involving either a renter or provider.
type Transaction struct {
	// Whether the transaction involved a renter or provider.
	UserType string `json:"userType"`
	// The ID of the associated user.
	UserID string `json:"userId"`
	// The contract associated with the transaction.
	ContractID string `json:"contractId"`
	// Whether the transction was a payment, receipt, deposit, or withdrawal.
	TransactionType string `json:"transactionType"`
	// The amount transferred, in tenths of cents.
	Amount int64 `json:"amount"`
	// A short description.
	Description string `json:"description"`
	// The time the transaction occurred.
	Date time.Time `json:"date"`
}

type BlockLocation struct {
	ProviderId string `json:"providerId"`
	Addr       string `json:"address"`
	ContractId string `json:"contractId"`
}

type Block struct {
	ID string `json:"id"`
	// Offset of the block in the file, relative to the file's other blocks.
	// For the first block, this is zero.
	Num int `json:"num"`
	// Size of the block in bytes
	Size int64 `json:"size"`
	// sha256 hash of the block
	Sha256Hash string `json:"sha256hash"`
	// Location of the provider where the block is stored
	Location BlockLocation `json:"location"`
}

// Permission provides access to a file to a non-owning user
type Permission struct {
	// The renter who this permission grants access to
	RenterId    string `json:"renterId"`
	RenterAlias string `json:"renterAlias"`
	// The file's encryption information encrypted with the user's public key
	AesKey string `json:"aesKey"`
	AesIV  string `json:"aesIV"`
}

type File struct {
	ID         string       `json:"id"`
	OwnerID    string       `json:"ownerId"`
	OwnerAlias string       `json:"ownerAlias"`
	Name       string       `json:"name"`
	IsDir      bool         `json:"isDir"`
	AccessList []Permission `json:"accessList"`
	AesKey     string       `json:"aesKey"`
	AesIV      string       `json:"aesIV"`
	Versions   []Version    `json:"versions"`
}

type Version struct {
	Num             int       `json:"num"`
	Size            int64     `json:"size"`
	ModTime         time.Time `json:"modTime"`
	UploadTime      time.Time `json:"uploadTime"`
	UploadSize      int64     `json:"uploadSize"`
	PaddingBytes    int64     `json:"paddingBytes"`
	NumDataBlocks   int       `json:"numDataBlocks"`
	NumParityBlocks int       `json:"numParityBlocks"`
	Blocks          []Block   `json:"blocks"`
}
