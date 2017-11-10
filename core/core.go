package core

type Provider struct {
	ID          string `json:id,omitempty`
	PublicKey   string `json:publicKey,omitempty`
	Host        string `json:host,omitempty`
	Port        int    `json:port,omitempty`
	SpaceAvail  int    `json:spaceAvail,omitempty`
	StorageRate int    `json:storageRate,omitempty`
}

type Block struct {
	ID        string     `json:id,omitempty`
	Locations []Provider `json:locations,omitempty`
}

type File struct {
	ID     string  `json:id,omitempty`
	Name   string  `json:name,omitempty`
	Blocks []Block `json:blocks,omitempty`
}

type Renter struct {
	ID        string `json:id,omitempty`
	PublicKey string `json:publicKey,omitempty`
	Files     []File `json:files,omitempty`
}
