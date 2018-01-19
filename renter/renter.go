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
	"crypto/aes"
	"crypto/rand"
	"github.com/satori/go.uuid"
	"io/ioutil"
	"net"
	"time"
	"compress/zlib"
	"crypto/cipher"
	"crypto/sha256"
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

	// All free storage blobs available for uploads.
	// Each provider with whom we have a contract
	// should have at most one blob in this list.
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

func (r *Renter) findBlobWithProvider(providerId string) (*storageBlob, bool) {
	for _, blob := range r.freelist {
		if blob.ProviderId == providerId {
			return blob, true
		}
	}
	return nil, false
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

		// Do we already have storage with this provider?
		blob, exists := r.findBlobWithProvider(signedContract.ProviderId)
		if exists {
			blob.Amount += contract.StorageSpace
		} else {
			blob = &storageBlob{
				ProviderId: pinfo.ID,
				Addr:       pinfo.Addr,
				Amount:     contract.StorageSpace,
			}
			r.freelist = append(r.freelist, blob)
		}
		break
	}
	if len(contracts) == 0 {
		return nil, errors.New("Cannot find storage providers")
	}

	err = r.saveSnapshot()
	if err != nil {
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

	// Find storage
	var blobIndices []int
	var blobs []*storageBlob
	var storageFound int64 = 0
	for idx, blob := range r.freelist {

		// Check if the provider is online
		conn, err := net.DialTimeout("tcp", blob.Addr, 3*time.Second)
		if err != nil {
			continue
		}
		conn.Close()
		blobIndices = append(blobIndices, idx)
		blobs = append(blobs, blob)
		storageFound += blob.Amount
		if storageFound >= finfo.Size() {
			break
		}
	}
	if storageFound < finfo.Size() {
		return nil, errors.New("Cannot find enough storage")
	}

	// Compress the file
	compressTemp, err := ioutil.TempFile("", "skybin_upload")
	if err != nil {
		return nil, errors.New("Unable to create temp file to prepare upload")
	}
	defer compressTemp.Close()
	defer os.Remove(compressTemp.Name())
	cw := zlib.NewWriter(compressTemp)
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("Unable to open file. Error: %s", err)
	}
	defer srcFile.Close()
	_, err = io.Copy(cw, srcFile)
	if err != nil {
		return nil, fmt.Errorf("Unable to compress file. Error: %s", err)
	}
	cw.Close()
	_, err = compressTemp.Seek(0, os.SEEK_SET)
	if err != nil {
		return nil, fmt.Errorf("Unable to seek to start of temp file. Error: %s", err)
	}

	// Encrypt
	aesKey := make([]byte, 32)
	_, err = rand.Reader.Read(aesKey)
	if err != nil {
		return nil, fmt.Errorf("Unable to create encryption key. Error: %s", err)
	}
	aesBlock, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("Unable to create block cipher for encryption. Error: %s", err)
	}
	aesIv := make([]byte, aes.BlockSize)
	_, err = rand.Reader.Read(aesIv)
	if err != nil {
		return nil, fmt.Errorf("Unable to read initialization vector. Error: %s", err)
	}
	stream := cipher.NewCFBEncrypter(aesBlock, aesIv)
	encryptTemp, err := ioutil.TempFile("", "skybin_upload")
	if err != nil {
		return nil, fmt.Errorf("Unable to create temp file for encrypting upload. Error: %s", err)
	}
	defer encryptTemp.Close()
	defer os.Remove(encryptTemp.Name())
	streamWriter := cipher.StreamWriter{S: stream, W: encryptTemp}
	_, err = io.Copy(streamWriter, compressTemp)
	if err != nil {
		return nil, fmt.Errorf("Unable to encrypt upload. Error: %s", err)
	}
	_, err = encryptTemp.Seek(0, os.SEEK_SET)
	if err != nil {
		return nil, fmt.Errorf("Unable to seek to start of temp file. Error: %s", err)
	}

	// Prepare blocks for upload
	var blocks []core.Block
	for _, blob := range blobs {
		lr := io.LimitReader(encryptTemp, blob.Amount)
		h := sha256.New()
		blockSize, err := io.Copy(h, lr)
		if err != nil {
			return nil, fmt.Errorf("Unable to hash block. Error: %s", err)
		}
		if blockSize == 0 {
			// EOF
			break
		}
		blockHash := string(h.Sum(nil))
		blockId, err := genId()
		if err != nil {
			return nil, fmt.Errorf("Unable to generate block ID. Error: %s", err)
		}
		blocks = append(blocks, core.Block{
			ID: blockId,
			Hash: blockHash,
			Size: blockSize,
			Locations: []core.BlockLocation{
				{ProviderId: blob.ProviderId, Addr: blob.Addr},
			},
		})
	}

	// Upload blocks
	_, err = encryptTemp.Seek(0, os.SEEK_SET)
	if err != nil {
		return nil, fmt.Errorf("Unable to seek file. Error: %s", err)
	}
	for _, block := range blocks {
		for _, location := range block.Locations {
			pvdr := provider.NewClient(location.Addr, &http.Client{})
			lr := io.LimitReader(encryptTemp, block.Size)
			err = pvdr.PutBlock(block.ID, r.Config.RenterId, lr)
			if err != nil {
				err = fmt.Errorf("Unable to upload block. Error: %s", err)
				break
			}
		}
	}
	if err != nil {
		// Upload failed. Unwind
		for _, block := range blocks {
			err := removeBlock(&block)
			if err != nil {
				// TODO: append block to list to be removed later...
			}
		}
		return nil, err
	}

	// We're done. Salvage remaining bytes of blobs that weren't all used
	tempInfo, err := encryptTemp.Stat()
	if err != nil {
		return nil, fmt.Errorf("Unable to stat encrypted file. Error: %s", err)
	}
	var bytesUsed int64 = 0
	for _, blob := range(blobs) {
		n := blob.Amount
		if n + bytesUsed > tempInfo.Size() {
			n = tempInfo.Size() - bytesUsed
			remaining := blob.Amount - n
			if remaining >= kMinBlobSize {
				r.freelist = append(r.freelist, &storageBlob{
					ProviderId: blob.ProviderId,
					Addr: blob.Addr,
					Amount: remaining,
				})
			}
		}
		bytesUsed += n
	}
	// And remove blobs that were used up
	for i := len(blobIndices) - 1; i >= 0; i-- {
		blobIdx := blobIndices[i]
		r.freelist = append(r.freelist[:blobIdx], r.freelist[blobIdx+1:]...)
	}

	fileId, err := genId()
	if err != nil {
		return nil, fmt.Errorf("Unable to generate file ID. Error: %s", err)
	}
	file := &core.File{
		ID:         fileId,
		Name:       destPath,
		IsDir:      false,
		Size:       finfo.Size(),
		ModTime:    finfo.ModTime(),
		AccessList: []core.Permission{},
		EncryptionKey: string(aesKey),
		EncryptionIV: string(aesIv),
		Blocks:     blocks,
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

	// Download
	encryptTemp, err := ioutil.TempFile("", "skybin_download")
	if err != nil {
		return fmt.Errorf("Unable to create temp file for download. Error: %s", err)
	}
	defer encryptTemp.Close()
	defer os.Remove(encryptTemp.Name())
	for _, block := range f.Blocks {
		err = downloadBlock(&block, encryptTemp)
		if err != nil {
			_ = os.Remove(destpath)
			return err
		}
	}
	_, err = encryptTemp.Seek(0, os.SEEK_SET)
	if err != nil {
		return fmt.Errorf("Unable to seek to beginning of temp file. Error: %s", err)
	}

	// Decrypt
	aesKey := []byte(f.EncryptionKey)
	aesCipher, err := aes.NewCipher(aesKey)
	if err != nil {
		return fmt.Errorf("Unable to create aes cipher. Error: %s", err)
	}
	iv := []byte(f.EncryptionIV)
	stream := cipher.NewCFBDecrypter(aesCipher, iv)
	streamReader := cipher.StreamReader{S: stream, R: encryptTemp}
	compressTemp, err := ioutil.TempFile("", "skybin_download")
	if err != nil {
		return fmt.Errorf("Unable to create temp file to decrypt download. Error: %s", err)
	}
	defer compressTemp.Close()
	defer os.Remove(compressTemp.Name())
	_, err = io.Copy(compressTemp, streamReader)
	if err != nil {
		return fmt.Errorf("Unable to decrypt file. Error: %s", err)
	}
	_, err = compressTemp.Seek(0, os.SEEK_SET)
	if err != nil {
		return fmt.Errorf("Unable to seek to beginning of decrypted temp. Error: %s", err)
	}

	// Decompress
	zr, err := zlib.NewReader(compressTemp)
	if err != nil {
		return fmt.Errorf("Unable to initialize decompression reader. Error: %s", err)
	}
	defer zr.Close()
	outFile, err := os.Create(destpath)
	if err != nil {
		return fmt.Errorf("Unable to create destination file. Error: %s", err)
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, zr)
	if err != nil {
		return fmt.Errorf("Unable to decompress file. Error: %s", err)
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
