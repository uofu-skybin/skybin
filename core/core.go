package core

import (
	"time"
)

const (
	DefaultMetaAddr     = "127.0.0.1:8001"
	DefaultRenterAddr   = "127.0.0.1:8002"
	DefaultProviderAddr = ":8003"
)

type ProviderInfo struct {
	ID          string `json:"id,omitempty"`
	PublicKey   string `json:"publicKey"`
	Addr        string `json:"address"`
	SpaceAvail  int64  `json:"spaceAvail,omitempty"`
	StorageRate int    `json:"storageRate,omitempty"`
}

type RenterInfo struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Files     []File `json:"files"`
}

type Contract struct {
	ID                string `json:"contractId"`
	RenterId          string `json:"renterId"`
	ProviderId        string `json:"providerId"`
	StorageSpace      int64  `json:"storageSpace"`
	RenterSignature   string `json:"renterSignature"`
	ProviderSignature string `json:"providerSignature"`
}

type BlockLocation struct {
	ProviderId string `json:"providerId"`
	Addr       string `json:"address"`
	ContractId string `json:"contractId"`
}

type Block struct {
	ID string `json:"id"`

	// sha256 hash of the block
	Sha256Hash string `json:"hash"`

	// Size of the block in bytes
	Size int64 `json:"size"`

	// Locations of providers where the block is stored
	Locations []BlockLocation `json:"locations"`
}

// Permission provides access to a file to a non-owning user
type Permission struct {

	// The renter who this permission grants access to
	RenterId string `json:"renterId"`

	// The file's encryption information encrypted with the user's public key
	AesKey string `json:"aesKey"`
	AesIV  string `json:"aesIV"`
}

type File struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	IsDir      bool         `json:"isDir"`
	Size       int64        `json:"size"`
	UploadSize int64        `json:"uploadSize"`
	ModTime    time.Time    `json:"modTime"`
	AccessList []Permission `json:"accessList"`
	AesKey     string       `json:"aesKey"`
	AesIV      string       `json:"aesIV"`
	Blocks     []Block      `json:"blocks"`
}

