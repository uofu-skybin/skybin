package renter

import (
	"errors"
	"fmt"
	"io"
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
	ApiAddr        string `json:"apiAddress"`
	MetaAddr       string `json:"metaServerAddress"`
	PrivateKeyFile string `json:"privateKeyFile"`
	PublicKeyFile  string `json:"publicKeyFile"`

	// Is this renter node registered with the metaservice?
	IsRegistered bool `json:"isRegistered"`
}

type Renter struct {
	Config    *Config
	Homedir   string
	files     []*core.File
	contracts []*core.Contract
	freelist  []*storageBlob
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
}

const (

	// The minimum size of a storage blob
	kMinBlobSize = 1
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

	return renter, err
}

// Info is information about a renter
type Info struct {
	ID              string `json:"id"`
	ReservedStorage int64  `json:"reservedStorage"`
	FreeStorage     int64  `json:"freeStorage"`
	TotalContracts  int    `json:"totalContracts"`
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
		ReservedStorage: reserved,
		FreeStorage:     free,
		TotalContracts:  len(r.contracts),
	}, nil
}

func (r *Renter) ReserveStorage(amount int64) ([]*core.Contract, error) {

	metaService := metaserver.NewClient(r.Config.MetaAddr, &http.Client{})
	providers, err := metaService.GetProviders()
	if err != nil {
		return nil, fmt.Errorf("Cannot fetch providers. Error: %s", err)
	}

	var contracts []*core.Contract
	for _, pinfo := range providers {
		contract := core.Contract{
			RenterId:     r.Config.RenterId,
			ProviderId:   pinfo.ID,
			StorageSpace: amount,
		}

		client := provider.NewClient(pinfo.Addr, &http.Client{})
		signedContract, err := client.ReserveStorage(&contract)
		if err != nil {
			continue
		}

		// Did the provider agree to the contract?
		if len(signedContract.ProviderSignature) == 0 {
			continue
		}

		contracts = append(contracts, signedContract)

		r.contracts = append(r.contracts, signedContract)
		r.freelist = append(r.freelist, &storageBlob{
			ProviderId: pinfo.ID,
			Addr:       pinfo.Addr,
			Amount:     contract.StorageSpace,
		})
		break
	}
	if len(contracts) == 0 {
		return nil, errors.New("Cannot find storage providers")
	}

	err = r.saveSnapshot()
	if err != nil {

		// Implicitly cancel any formed contracts.
		return nil, fmt.Errorf("Unable to save snapshot. Error: %s", err)
	}

	return contracts, nil
}

func (r *Renter) CreateFolder(name string) (*core.File, error) {
	id, err := genId()
	if err != nil {
		return nil, fmt.Errorf("Cannot generate folder ID. Error: %s", err)
	}
	file := &core.File{
		ID:         id,
		Name:       name,
		IsDir:      true,
		AccessList: []core.Permission{},
		Blocks:     []core.Block{},
	}
	err = r.addFile(file)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (r *Renter) Upload(srcPath, destPath string) (*core.File, error) {
	finfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	if finfo.IsDir() {
		return nil, errors.New("Folder uploads not supported yet")
	}
	if finfo.Size() > (1 << 28) {
		return nil, errors.New("Large file uploads not supported yet")
	}

	// Find storage
	var blobIdx int
	var blob *storageBlob
	for blobIdx, blob = range r.freelist {
		if blob.Amount >= finfo.Size() {
			break
		}
	}
	if blob == nil {
		return nil, errors.New("Cannot find enough storage. " +
			"Be sure to reserve storage before uploading files.")
	}

	// Upload the file to the provider
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("Cannot open file. Error: %s", err)
	}
	defer f.Close()

	blockId, err := genId()
	if err != nil {
		return nil, fmt.Errorf("Cannot generate block ID. Error: %s", err)
	}

	pvdr := provider.NewClient(blob.Addr, &http.Client{})
	err = pvdr.PutBlock(blockId, r.Config.RenterId, f)
	if err != nil {
		return nil, fmt.Errorf("Cannot upload block to provider. Error: %s", err)
	}

	// Remove the used storage from the freelist
	r.freelist = append(r.freelist[:blobIdx], r.freelist[blobIdx+1:]...)
	remaining := blob.Amount - finfo.Size()
	if remaining > kMinBlobSize {
		leftover := &storageBlob{
			ProviderId: blob.ProviderId,
			Addr:       blob.Addr,
			Amount:     remaining,
		}
		r.freelist = append(r.freelist, leftover)
	}

	block := core.Block{
		ID: blockId,
		Locations: []core.BlockLocation{
			{ProviderId: blob.ProviderId, Addr: blob.Addr},
		},
	}

	fileId, err := genId()
	if err != nil {
		return nil, fmt.Errorf("Cannot generate file ID. Error: %s", err)
	}
	file := &core.File{
		ID:         fileId,
		Name:       destPath,
		IsDir:      false,
		Size:       finfo.Size(),
		ModTime:    finfo.ModTime(),
		AccessList: []core.Permission{},
		Blocks: []core.Block{
			block,
		},
	}

	err = r.addFile(file)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (r *Renter) ListFiles() ([]*core.File, error) {
	return r.files, nil
}

func (r *Renter) Lookup(fileId string) (*core.File, error) {
	_, f := r.findFile(fileId)
	if f == nil {
		return nil, fmt.Errorf("Cannot find file with ID %s", fileId)
	}
	return f, nil
}

func (r *Renter) Download(f *core.File, destpath string) error {
	if f.IsDir {
		return errors.New("Folder downloads not supported yet")
	}

	outFile, err := os.Create(destpath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for _, block := range f.Blocks {
		err = downloadBlock(&block, outFile)
		if err != nil {
			_ = os.Remove(destpath)
			return err
		}
	}

	return nil
}

func (r *Renter) ShareFile(f *core.File, userId string) error {
	f.AccessList = append(f.AccessList, core.Permission{
		UserId: userId,
	})
	err := r.saveSnapshot()
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
	r.files = append(r.files[:idx], r.files[idx+1:]...)
	err := r.saveSnapshot()
	if err != nil {
		return fmt.Errorf("Unable to save snapshot. Error: %s", err)
	}
	for _, block := range f.Blocks {
		err := removeBlock(&block)
		if err != nil {
			return fmt.Errorf("Could not delete block %s. Error: %s", block.ID, err)
		}

	}
	return nil
}

func (r *Renter) addFile(f *core.File) error {
	r.files = append(r.files, f)
	err := r.saveSnapshot()
	if err != nil {
		r.files = r.files[:len(r.files)-1]
		return fmt.Errorf("Unable to save snapshot. Error %s", err)
	}
	return nil
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

func downloadBlock(block *core.Block, out io.Writer) error {
	for _, location := range block.Locations {
		client := provider.NewClient(location.Addr, &http.Client{})

		blockReader, err := client.GetBlock(block.ID)
		if err != nil {

			// TODO: Check that failure is due to a network error, not because
			// provider didn't return the block.
			continue
		}
		defer blockReader.Close()

		_, err = io.Copy(out, blockReader)
		if err != nil {
			return fmt.Errorf("Cannot write block %s to local file. Error: %s", block.ID, err)
		}

		return nil
	}
	return fmt.Errorf("Unable to download file block %s. Cannot connect to providers.", block.ID)
}

func removeBlock(block *core.Block) error {
	for _, location := range block.Locations {
		pvdr := provider.NewClient(location.Addr, &http.Client{})
		err := pvdr.RemoveBlock(block.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func genId() (string, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
