package renter

import (
	"errors"
	"fmt"
	"github.com/satori/go.uuid"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"skybin/metaserver"
	"skybin/provider"
	"skybin/util"
)

type Config struct {
	RenterId     string `json:"renterId"`
	Addr         string `json:"address"`
	MetaAddr     string `json:"metaServerAddress"`
	IdentityFile string `json:"identityFile"`
}

type Renter struct {
	Config    *Config
	Homedir   string
	files     []core.File
	contracts []core.Contract
	freelist  []storageBlob
}

// snapshot stores a renter's serialized state
type snapshot struct {
	Files []core.File `json:"files"`
	Contracts []core.Contract `json:"contracts"`
	FreeStorage []storageBlob `json:"freeStorage"`
}

// storageBlob is a chunk of free storage we've already rented
type storageBlob struct {
	ProviderId string // The provider who owns the rented storage
	Addr       string // The provider's network address
	Amount     int64 // The free storage in bytes
}

const (

	// The minimum size of a storage blob
	kMinBlobSize = 1
)

func LoadFromDisk(homedir string) (*Renter, error) {
	renter := &Renter{
		Homedir: homedir,
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

func (r *Renter) saveSnapshot() error {
	s := snapshot{
		Files: r.files,
		Contracts: r.contracts,
		FreeStorage: r.freelist,
	}
	return util.SaveJson(path.Join(r.Homedir, "snapshot.json"), &s)
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

		r.contracts = append(r.contracts, *signedContract)
		r.freelist = append(r.freelist, storageBlob{
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

func (r *Renter) Upload(srcPath, destPath string) (*core.File, error) {
	finfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	if finfo.IsDir() {
		return nil, errors.New("Directory uploads not supported yet")
	}
	if finfo.Size() > (1 << 28) {
		return nil, errors.New("Large file uploads not supported yet")
	}

	// Find storage
	var blobIdx int
	var blob storageBlob
	for blobIdx, blob = range r.freelist {
		if blob.Amount >= finfo.Size() {
			break
		}
	}
	if blob.Amount < finfo.Size() {
		return nil, errors.New("Cannot find enough storage. " +
			"Be sure to reserve storage before uploading files.")
	}

	// Upload file to provider
	data, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("Cannot read file. Error: %s", err)
	}
	blockId := core.Hash(data)
	provider := provider.NewClient(blob.Addr, &http.Client{})
	err = provider.PutBlock(blockId, data)
	if err != nil {
		return nil, fmt.Errorf("Cannot upload block to provider. Error: %s", err)
	}

	// Remove the used storage from the freelist
	r.freelist = append(r.freelist[:blobIdx], r.freelist[blobIdx +1:]...)
	remaining := blob.Amount - finfo.Size()
	if remaining > kMinBlobSize {
		leftover := storageBlob{
			ProviderId: blob.ProviderId,
			Addr: blob.Addr,
			Amount: remaining,
		}
		r.freelist = append(r.freelist, leftover)
	}

	// Save the file metadata
	file := core.File{
		ID:   uuid.NewV4().String(),
		Name: destPath,
	}
	file.Blocks = append(file.Blocks, core.Block{
		ID: blockId,
		Locations: []core.BlockLocation{
			{ProviderId: blob.ProviderId, Addr: blob.Addr},
		},
	})
	r.files = append(r.files, file)

	err = r.saveSnapshot()
	if err != nil {

		// TODO: We probably shouldn't bail in this case.
		return nil, fmt.Errorf("Unable to save snapshot. Error: %s", err)
	}

	return &file, nil
}

func (r *Renter) ListFiles() ([]core.File, error) {
	return r.files, nil
}

func (r *Renter) Lookup(fileId string) (*core.File, error) {
	for _, file := range r.files {
		if file.ID == fileId {
			return &file, nil
		}
	}
	return nil, fmt.Errorf("Cannot find file with ID %s", fileId)
}

func (r *Renter) Download(fileInfo *core.File, destpath string) error {
	outFile, err := os.Create(destpath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for _, block := range fileInfo.Blocks {
		err = downloadBlock(&block, outFile)
		if err != nil {
			_ = os.Remove(destpath)
			return err
		}
	}

	return nil
}

func downloadBlock(block *core.Block, out io.Writer) error {
	for _, location := range block.Locations {
		client := provider.NewClient(location.Addr, &http.Client{})

		data, err := client.GetBlock(block.ID)
		if err != nil {

			// TODO: Check that failure is due to a network error, not because
			// provider didn't return the block.
			continue
		}

		_, err = out.Write(data)
		if err != nil {
			return fmt.Errorf("Cannot write block %s to local file. Error: %s", block.ID, err)
		}

		return nil
	}
	return fmt.Errorf("Unable to download file block %s. Cannot connect to providers.", block.ID)
}
