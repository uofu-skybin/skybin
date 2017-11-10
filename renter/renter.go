package renter

import (
	"crypto/sha1"
	"encoding/base32"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"skybin/core"
	"skybin/metaserver"
	"skybin/provider"
)

func hash(data []byte) string {
	h := sha1.New()
	h.Write(data)
	return base32.StdEncoding.EncodeToString(h.Sum(nil))
}

type Config struct {
	Addr         string `json:"address"`
	MetaAddr     string `json:"metaServerAddress"`
	IdentityFile string `json:"identityFile"`
}

type Renter struct {
	Config    *Config
	Homedir   string
	contracts []core.Contract
	freelist  []storageBlob
	files     []core.File
}

type storageBlob struct {
	ProviderID string
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
			RenterID:     "",
			ProviderID:   "",
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
			ProviderID: pinfo.ID,
			Addr:       pinfo.Addr,
			Amount:     contract.StorageSpace,
		})
		return nil
	}

	return errors.New("cannot find storage provider")
}

func (r *Renter) Upload(srcPath, destPath string) error {
	finfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if finfo.IsDir() {
		return errors.New("directory uploads not supported yet")
	}

	var idx int
	var blob storageBlob
	for idx, blob = range r.freelist {
		if blob.Amount >= finfo.Size() {
			break
		}
	}
	if blob.Amount < finfo.Size() {
		return errors.New("error: not enough storage")
	}

	data, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("error: cannot read file. error: %s", err)
	}
	blockID := hash(data)

	provider := provider.NewClient(blob.Addr, &http.Client{})
	err = provider.PutBlock(blockID, data)
	if err != nil {
		return fmt.Errorf("error: cannot upload block. error: %s", err)
	}

	r.freelist = append(r.freelist[:idx], r.freelist[idx+1:]...)
	r.files = append(r.files, core.File{
		Name: destPath,
		Blocks: []core.Block{
			{ID: blockID, Locations: []string{blob.ProviderID}},
		},
	})

	return nil
}
