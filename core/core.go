package core

const (
	DefaultMetaAddr   = "127.0.0.1:8001"
	DefaultRenterAddr = "127.0.0.1:8002"
)

type Provider struct {
	ID          string `json:"id,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
	Addr        string  `json:"address"`
	SpaceAvail  int64    `json:"spaceAvail,omitempty"`
	StorageRate int    `json:"storageRate,omitempty"`
}

type Contract struct {
	RenterID          string `json:"renterID"`
	ProviderID        string `json:"providerID"`
	StorageSpace      int    `json:"storageSpace"`
	RenterSignature   string `json:"renterSignature"`
	ProviderSignature string `json:"providerSignature"`
}

type Block struct {
	ID        string     `json:"id,omitempty"`
	Locations []Provider `json:"locations,omitempty"`
}

type File struct {
	ID     string  `json:"id,omitempty"`
	Name   string  `json:"name,omitempty"`
	Blocks []Block `json:"blocks,omitempty"`
}

type Renter struct {
	ID        string `json:"id,omitempty"`
	PublicKey string `json:"publicKey,omitempty"`
	Files     []File `json:"files,omitempty"`
}
