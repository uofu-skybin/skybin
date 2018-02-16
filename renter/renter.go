package renter

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"skybin/metaserver"
	"skybin/provider"
	"skybin/util"
	"strings"

	"github.com/satori/go.uuid"
)

type Config struct {
	RenterId       string `json:"renterId"`
	Alias          string `json:"alias"`
	ApiAddr        string `json:"apiAddress"`
	MetaAddr       string `json:"metaServerAddress"`
	PrivateKeyFile string `json:"privateKeyFile"`
	PublicKeyFile  string `json:"publicKeyFile"`

	// Is this renter node registered with the metaservice?
	IsRegistered bool `json:"isRegistered"`
}

type Renter struct {
	Config     *Config
	Homedir    string
	files      []*core.File
	contracts  []*core.Contract
	privKey    *rsa.PrivateKey
	metaClient *metaserver.Client

	// All free storage blobs available for uploads.
	// Each storage contract should have at most one associated
	// blob in this list.
	freelist []*storageBlob
}

// snapshot stores a renter's serialized state
type snapshot struct {
	Files       []*core.File     `json:"files"`
	Contracts   []*core.Contract `json:"contracts"`
	FreeStorage []*storageBlob   `json:"freeStorage"`
}

// storageBlob is a chunk of free storage we've already rented
type storageBlob struct {
	ProviderId string // The provider who owns the rented storage
	Addr       string // The provider's network address
	Amount     int64  // The free storage in bytes
	ContractId string // The contract the blob is associated with
}

const (

	// The minimum size of a storage blob
	kMinBlobSize = 1

	// Minimum contract storage amount
	// A user cannot reserve less storage than this
	kMinContractSize = 1024 * 1024

	// Maximum storage amount of any contract
	kMaxContractSize = 1024 * 1024 * 1024

	// Maximum size of an uploaded block
	kMaxBlockSize = kMaxContractSize

	// Erasure encoding defaults
	kDefaultDataBlocks   = 8
	kDefaultParityBlocks = 4
)

func LoadFromDisk(homedir string) (*Renter, error) {
	renter := &Renter{
		Homedir:   homedir,
		files:     make([]*core.File, 0),
		contracts: make([]*core.Contract, 0),
		freelist:  make([]*storageBlob, 0),
	}

	config := &Config{}
	err := util.LoadJson(path.Join(homedir, "config.json"), config)
	if err != nil {
		return nil, err
	}
	renter.Config = config

	metaHTTPClient := http.Client{}
	renter.metaClient = metaserver.NewClient(config.MetaAddr, &metaHTTPClient)

	snapshotPath := path.Join(homedir, "snapshot.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		var s snapshot
		err := util.LoadJson(snapshotPath, &s)
		if err != nil {
			return nil, fmt.Errorf("Unable to load snapshot. Error: %s", err)
		}
		renter.files = s.Files
		renter.contracts = s.Contracts
		renter.freelist = s.FreeStorage
	}

	privKey, err := loadPrivateKey(path.Join(homedir, "renterid"))
	if err != nil {
		return nil, err
	}
	renter.privKey = privKey

	return renter, err
}

func (r *Renter) authorize() error {
	privKey, err := loadPrivateKey(r.Config.PrivateKeyFile)
	if err != nil {
		return err
	}

	err = r.metaClient.AuthorizeRenter(privKey, r.Config.RenterId)
	if err != nil {
		return err
	}

	return nil
}

// Info is information about a renter
type Info struct {
	ID              string `json:"id"`
	Alias           string `json:"alias"`
	ApiAddr         string `json:"apiAddress"`
	HomeDir         string `json:"homedir"`
	ReservedStorage int64  `json:"reservedStorage"`
	FreeStorage     int64  `json:"freeStorage"`
	UsedStorage     int64  `json:"usedStorage"`
	TotalContracts  int    `json:"totalContracts"`
	TotalFiles      int    `json:"totalFiles"`
}

func (r *Renter) Info() (*Info, error) {
	var reserved int64 = 0
	for _, contract := range r.contracts {
		reserved += contract.StorageSpace
	}
	var free int64 = 0
	for _, blob := range r.freelist {
		free += blob.Amount
	}
	return &Info{
		ID:              r.Config.RenterId,
		Alias:           r.Config.Alias,
		ApiAddr:         r.Config.ApiAddr,
		HomeDir:         r.Homedir,
		ReservedStorage: reserved,
		UsedStorage:     reserved - free,
		FreeStorage:     free,
		TotalContracts:  len(r.contracts),
		TotalFiles:      len(r.files),
	}, nil
}

func (r *Renter) CreateFolder(name string) (*core.File, error) {
	id, err := genId()
	if err != nil {
		return nil, fmt.Errorf("Cannot generate folder ID. Error: %s", err)
	}
	file := &core.File{
		ID:         id,
		OwnerID:    r.Config.RenterId,
		Name:       name,
		IsDir:      true,
		AccessList: []core.Permission{},
		Versions:   []core.Version{},
	}
	err = r.saveFile(file)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (r *Renter) RenameFile(fileId string, name string) (*core.File, error) {
	file, err := r.Lookup(fileId)
	if err != nil {
		return nil, err
	}
	for _, file2 := range r.files {
		if file2.Name == name {
			return nil, errors.New("Cannot rename file. Name already taken")
		}
	}
	if file.IsDir {
		children := r.findChildren(file)
		for _, child := range children {
			suffix := strings.TrimPrefix(child.Name, file.Name)
			child.Name = name + suffix
		}
	}
	file.Name = name
	err = r.saveSnapshot()
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (r *Renter) ListFiles() ([]*core.File, error) {
	return r.files, nil
}

func (r *Renter) ListSharedFiles() ([]*core.File, error) {
	if !r.metaClient.IsAuthorized() {
		r.authorize()
	}

	files, err := r.metaClient.GetSharedFiles(r.Config.RenterId)
	if err != nil {
		return nil, err
	}

	// Do this to match ListFile's signature, for now.
	returnList := make([]*core.File, len(files))
	for i, f := range files {
		returnList[i] = &f
	}

	return returnList, nil
}

func (r *Renter) Lookup(fileId string) (*core.File, error) {
	_, f := r.findFile(fileId)
	if f != nil {
		return f, nil
	}

	// If we couldn't find the file locally, check if it is a shared file.
	if !r.metaClient.IsAuthorized() {
		r.authorize()
	}

	f, err := r.metaClient.GetFile(r.Config.RenterId, fileId)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (r *Renter) ShareFile(fileId string, renterAlias string) error {
	f, err := r.Lookup(fileId)
	if err != nil {
		return err
	}

	// Get the renter's information
	renterInfo, err := r.metaClient.GetRenterByAlias(renterAlias)
	if err != nil {
		return err
	}

	// Encrypt the AES key and IV with the renter's public key
	pubKey, err := util.UnmarshalPublicKey([]byte(renterInfo.PublicKey))
	if err != nil {
		return err
	}

	decryptedKey, decryptedIV, err := r.decryptEncryptionKeys(f)
	if err != nil {
		return err
	}

	rng := rand.Reader
	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), rng, pubKey, decryptedKey, nil)
	encryptedIV, err := rsa.EncryptOAEP(sha256.New(), rng, pubKey, decryptedIV, nil)

	permission := core.Permission{
		RenterId: renterInfo.ID,
		AesKey:   base64.URLEncoding.EncodeToString(encryptedKey),
		AesIV:    base64.URLEncoding.EncodeToString(encryptedIV),
	}

	if !r.metaClient.IsAuthorized() {
		err = r.authorize()
		if err != nil {
			return err
		}
	}

	err = r.metaClient.ShareFile(r.Config.RenterId, f.ID, &permission)
	if err != nil {
		return err
	}

	f.AccessList = append(f.AccessList, permission)
	err = r.saveSnapshot()
	if err != nil {
		return fmt.Errorf("Unable to save snapshot. Error %s", err)
	}
	return nil
}

func (r *Renter) Remove(fileId string) error {
	idx, f := r.findFile(fileId)
	if f == nil {
		return fmt.Errorf("Cannot find file with ID %s", fileId)
	}
	if f.IsDir && len(r.findChildren(f)) > 0 {
		return errors.New("Cannot remove non-empty folder")
	}
	for _, version := range f.Versions {
		r.removeVersion(&version)
	}
	r.files = append(r.files[:idx], r.files[idx+1:]...)
	err := r.saveSnapshot()
	if err != nil {
		return fmt.Errorf("Unable to save snapshot. Error: %s", err)
	}
	return nil
}

func (r *Renter) removeVersion(version *core.Version) {
	for _, block := range version.Blocks {
		err := r.removeBlock(&block)
		if err != nil {
			// TODO: add to list to remove later
			continue
		}
		for _, location := range block.Locations {
			blob := &storageBlob{
				ProviderId: location.ProviderId,
				Addr:       location.Addr,
				Amount:     block.Size,
				ContractId: location.ContractId,
			}
			r.addBlob(blob)
		}
	}
}

func (r *Renter) removeBlock(block *core.Block) error {
	for _, location := range block.Locations {
		pvdr := provider.NewClient(location.Addr, &http.Client{})
		err := pvdr.RemoveBlock(r.Config.RenterId, block.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// Add a storage blob back to the free list.
func (r *Renter) addBlob(blob *storageBlob) {
	for _, blob2 := range r.freelist {
		if blob.ContractId == blob2.ContractId {

			// Merge
			blob2.Amount += blob.Amount
			return
		}
	}
	r.freelist = append(r.freelist, blob)
}

func (r *Renter) saveFile(f *core.File) error {
	r.files = append(r.files, f)
	return r.saveSnapshot()
}

func (r *Renter) findFile(fileId string) (idx int, f *core.File) {
	for idx, f = range r.files {
		if f.ID == fileId {
			return
		}
	}
	return -1, nil
}

func (r *Renter) findChildren(dir *core.File) []*core.File {
	var children []*core.File
	for _, f := range r.files {
		if f != dir && strings.HasPrefix(f.Name, dir.Name) {
			children = append(children, f)
		}
	}
	return children
}

func (r *Renter) saveSnapshot() error {
	s := snapshot{
		Files:       r.files,
		Contracts:   r.contracts,
		FreeStorage: r.freelist,
	}
	return util.SaveJson(path.Join(r.Homedir, "snapshot.json"), &s)
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return util.UnmarshalPrivateKey(data)
}

func genId() (string, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
