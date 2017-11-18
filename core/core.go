package core

const (
	DefaultMetaAddr     = "127.0.0.1:8001"
	DefaultRenterAddr   = "127.0.0.1:8002"
	DefaultProviderAddr = ":8003"
)

type Provider struct {
	ID          string `json:"id,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
	Addr        string `json:"address"`
	SpaceAvail  int64  `json:"spaceAvail,omitempty"`
	StorageRate int    `json:"storageRate,omitempty"`
}

type Contract struct {
	RenterId          string `json:"renterID"`
	ProviderId        string `json:"providerID"`
	StorageSpace      int64  `json:"storageSpace"`
	RenterSignature   string `json:"renterSignature"`
	ProviderSignature string `json:"providerSignature"`
}

type BlockLocation struct {
	ProviderId string `json:"providerId"`
	Addr       string `json:"address"`
}

type Block struct {
	ID        string          `json:"id"`
	Locations []BlockLocation `json:"locations"`
}

type File struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	IsDir  bool    `json:"isDir"`
	Blocks []Block `json:"blocks"`
}

type Renter struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Files     []File `json:"files"`
}
