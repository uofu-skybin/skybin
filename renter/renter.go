package renter

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"skybin/core"
	"skybin/metaserver"
	"skybin/provider"
	"github.com/satori/go.uuid"
	"io"
	"log"
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

type storageBlob struct {
	ProviderId string
	Addr       string
	Amount     int64
}

func (r *Renter) ReserveStorage(amount int64) error {
	metaService := metaserver.NewClient(r.Config.MetaAddr, &http.Client{})
	providers, err := metaService.GetProviders()
	if err != nil {
		return err
	}

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
		if len(signedContract.ProviderSignature) == 0 {
			continue
		}
		r.contracts = append(r.contracts, contract)
		r.freelist = append(r.freelist, storageBlob{
			ProviderId: pinfo.ID,
			Addr:       pinfo.Addr,
			Amount:     contract.StorageSpace,
		})
		return nil
	}

	return errors.New("cannot find storage provider")
}

func (r *Renter) Upload(srcPath, destPath string) (*core.File, error) {
	finfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	if finfo.IsDir() {
		return nil, errors.New("directory uploads not supported yet")
	}

	var idx int
	var blob storageBlob
	for idx, blob = range r.freelist {
		if blob.Amount >= finfo.Size() {
			break
		}
	}
	if blob.Amount < finfo.Size() {
		return nil, errors.New("error: not enough storage")
	}

	data, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("error: cannot read file. error: %s", err)
	}
	blockId := core.Hash(data)

	provider := provider.NewClient(blob.Addr, &http.Client{})
	err = provider.PutBlock(blockId, data)
	if err != nil {
		return nil, fmt.Errorf("error: cannot upload block. error: %s", err)
	}
	r.freelist = append(r.freelist[:idx], r.freelist[idx+1:]...)

	file := core.File{
		ID: uuid.NewV4().String(),
		Name: destPath,
	}
	file.Blocks = append(file.Blocks, core.Block{
		ID: blockId,
		Locations: []core.BlockLocation{
			{ProviderId: blob.ProviderId, Addr: blob.Addr},
		},
	})
	r.files = append(r.files, file)

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
	return nil, fmt.Errorf("cannot find file with ID %s", fileId)
}

func (r *Renter) Download(fileInfo *core.File, destpath string) error {

	log.Println("downloading to", destpath)

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
			continue
		}
		// TODO: Check short write.
		_, err = out.Write(data)
		return err
	}
	return fmt.Errorf("cannot download block %s", block.ID)
}
