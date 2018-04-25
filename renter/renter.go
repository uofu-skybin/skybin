package renter

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"skybin/metaserver"
	"skybin/provider"
	"skybin/util"
	"strings"
	"sync"
	"time"
)

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
	Balance         int64  `json:"balance"`
}

type Renter struct {
	Config  *Config
	Homedir string
	privKey *rsa.PrivateKey

	// An in-memory cache of the renter's file metadata.
	files []*core.File

	// The last time we pulled down a list of our files from the metaserver.
	lastFilesUpdate time.Time

	// Metaserver client. The metaserver should always have an up-to-date
	// view of every file and contract we store locally. However, the
	// opposite is not necessarily true; we may not have an up-to-date
	// view of everything the metaserver stores on our behalf.
	metaClient     *metaserver.Client

	storageManager *storageManager

	// Blocks which need to be removed but could not be immediately
	// deleted because the provider storing them was offline.
	blocksToDelete []*core.Block

	// Queue for batches of downloads to be performed by the download thread.
	downloadQ      chan []*fileDownload
	uploadQ        chan *fileUpload
	restoreQ       chan *recoveredBlockBatch
	logger         *log.Logger
	mu             sync.RWMutex
}

// snapshot stores a renter's serialized state
type snapshot struct {
	Files          []*core.File     `json:"files"`
	Contracts      []*core.Contract `json:"contracts"`
	FreeStorage    []*storageBlob   `json:"freeStorage"`
	BlocksToDelete []*core.Block    `json:"blocksToDelete"`
}

// storageBlob is a chunk of free storage we've already rented
type storageBlob struct {
	ProviderId string // The provider who owns the rented storage
	Addr       string // The provider's network address
	Amount     int64  // The free storage in bytes
	ContractId string // The contract the blob is associated with
}

var (
	// Indicates that a block download has failed because
	// the block contents are incorrect.
	errBlockCorrupted = errors.New("Block corrupted")

	// Default HTTP client to use for requests to providers
	netClient = &http.Client{
		Timeout: 20 * time.Second,
	}
)

func LoadFromDisk(homedir string) (*Renter, error) {

	renter := &Renter{
		Homedir:        homedir,
		files:          make([]*core.File, 0),
		blocksToDelete: make([]*core.Block, 0),
		downloadQ:      make(chan []*fileDownload),
		uploadQ:        make(chan *fileUpload),
		restoreQ:      make(chan *recoveredBlockBatch),
		logger:         log.New(ioutil.Discard, "", log.LstdFlags),
	}

	config := &Config{}
	err := util.LoadJson(path.Join(homedir, "config.json"), config)
	if err != nil {
		return nil, err
	}
	renter.Config = config

	renter.metaClient = metaserver.NewClient(config.MetaAddr, &http.Client{})

	updateStorage := func() ([]*storageBlob, error) {
		// TODO: implement
		return nil, errors.New("")
	}
	renter.storageManager = newStorageManager([]*storageBlob{}, updateStorage, time.Hour, realClock{})

	snapshotPath := path.Join(homedir, "snapshot.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		var s snapshot
		err := util.LoadJson(snapshotPath, &s)
		if err != nil {
			return nil, fmt.Errorf("Unable to load snapshot. Error: %s", err)
		}
		renter.files = s.Files
		renter.blocksToDelete = s.BlocksToDelete

		renter.storageManager.AddBlobs(s.FreeStorage)
	}

	privKey, err := loadPrivateKey(path.Join(homedir, "renterid"))
	if err != nil {
		return nil, err
	}
	renter.privKey = privKey

	return renter, err
}

func (r *Renter) StartBackgroundThreads() {
	go r.downloadThread()
	go r.uploadThread()
	go r.blockRestoreThread()
}

func (r *Renter) ShutdownThreads() {
	close(r.downloadQ)
	close(r.uploadQ)
	close(r.restoreQ)
}

func (r *Renter) SetLogger(logger *log.Logger) {
	r.logger = logger
}

func (r *Renter) saveSnapshot() error {
	s := snapshot{
		Files: r.files,
		//Contracts: r.contracts,

		// TODO: remove this from snapshot
		FreeStorage:    r.storageManager.freelist,
		BlocksToDelete: r.blocksToDelete,
	}
	return util.SaveJson(path.Join(r.Homedir, "snapshot.json"), &s)
}

func (r *Renter) Info() (*Info, error) {
	err := r.authorizeMeta()
	if err != nil {
		return nil, err
	}
	renterInfo, err := r.metaClient.GetRenter(r.Config.RenterId)
	if err != nil {
		return nil, err
	}
	contracts, err := r.metaClient.GetRenterContracts(r.Config.RenterId)
	if err != nil {
		return nil, err
	}
	var reserved int64 = 0
	for _, contract := range contracts {
		reserved += contract.StorageSpace
	}
	freeStorage := r.storageManager.AvailableStorage()
	r.mu.RLock()
	numFiles := len(r.files)
	r.mu.RUnlock()
	return &Info{
		ID:              r.Config.RenterId,
		Alias:           r.Config.Alias,
		ApiAddr:         r.Config.ApiAddr,
		HomeDir:         r.Homedir,
		ReservedStorage: reserved,
		UsedStorage:     reserved - freeStorage,
		FreeStorage:     freeStorage,
		TotalContracts:  len(contracts),
		TotalFiles:      numFiles,
		Balance:         renterInfo.Balance,
	}, nil
}

func (r *Renter) CreateFolder(name string) (*core.File, error) {
	if _, err := r.GetFileByName(name); err == nil {
		return nil, fmt.Errorf("%s already exists.", name)
	}
	id, err := util.GenerateID()
	if err != nil {
		return nil, fmt.Errorf("Cannot generate folder ID. Error: %s", err)
	}
	file := &core.File{
		ID:         id,
		OwnerID:    r.Config.RenterId,
		OwnerAlias: r.Config.Alias,
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

func (r *Renter) ListFiles() ([]*core.File, error) {
	if time.Now().After(r.lastFilesUpdate.Add(5 * time.Minute)) {
		err := r.pullFiles()
		if err != nil {
			r.logger.Println("Unable to refresh file metadata cache. Error: ", err)
		}
	}
	return r.files, nil
}

// Refreshes the local list of files from the metaserver
func (r *Renter) pullFiles() error {
	err := r.authorizeMeta()
	if err != nil {
		return err
	}

	files, err := r.metaClient.GetFiles(r.Config.RenterId)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.files = files
	r.mu.Unlock()
	r.lastFilesUpdate = time.Now()
	return r.saveSnapshot()
}

func (r *Renter) GetFile(fileId string) (*core.File, error) {

	// Check if it exists locally
	r.mu.RLock()
	for _, file := range r.files {
		if file.ID == fileId {
			r.mu.RUnlock()
			return file, nil
		}
	}
	r.mu.RUnlock()

	// It might not exist because the local cache is out of date.
	// Check with the metaserver.
	err := r.authorizeMeta()
	if err != nil {
		return nil, err
	}
	file, err := r.metaClient.GetFile(r.Config.RenterId, fileId)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.files = append(r.files, file)
	r.mu.Unlock()

	return file, nil
}

func (r *Renter) ListSharedFiles() ([]*core.File, error) {
	err := r.authorizeMeta()
	if err != nil {
		return nil, err
	}

	files, err := r.metaClient.GetSharedFiles(r.Config.RenterId)
	if err != nil {
		return nil, err
	}

	// Do this to match ListFile's signature, for now.
	returnList := make([]*core.File, len(files))
	for i, f := range files {
		newFile := f
		returnList[i] = &newFile
	}

	return returnList, nil
}

func (r *Renter) ShareFile(fileId string, renterAlias string) error {
	file, err := r.GetFile(fileId)
	if err != nil {
		return err
	}

	if file.IsDir {
		return errors.New("Folder sharing not supported")
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

	decryptedKey, decryptedIV, err := r.decryptEncryptionKeys(file)
	if err != nil {
		return err
	}

	rng := rand.Reader
	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), rng, pubKey, decryptedKey, nil)
	encryptedIV, err := rsa.EncryptOAEP(sha256.New(), rng, pubKey, decryptedIV, nil)

	permission := core.Permission{
		RenterId:    renterInfo.ID,
		RenterAlias: renterAlias,
		AesKey:      base64.URLEncoding.EncodeToString(encryptedKey),
		AesIV:       base64.URLEncoding.EncodeToString(encryptedIV),
	}

	err = r.authorizeMeta()
	if err != nil {
		return err
	}

	err = r.metaClient.ShareFile(r.Config.RenterId, file.ID, &permission)
	if err != nil {
		return err
	}

	file.AccessList = append(file.AccessList, permission)
	err = r.saveSnapshot()
	if err != nil {
		return fmt.Errorf("Unable to save snapshot. Error %s", err)
	}
	return nil
}

// Decrypts and returns f's AES key and AES IV.
func (r *Renter) decryptEncryptionKeys(f *core.File) (aesKey []byte, aesIV []byte, err error) {
	var keyToDecrypt string
	var ivToDecrypt string

	// If we own the file, use the AES key directly. Otherwise, retrieve them from the relevent permission
	if f.OwnerID == r.Config.RenterId {
		keyToDecrypt = f.AesKey
		ivToDecrypt = f.AesIV
	} else {
		for _, permission := range f.AccessList {
			if permission.RenterId == r.Config.RenterId {
				keyToDecrypt = permission.AesKey
				ivToDecrypt = permission.AesIV
			}
		}
	}

	if keyToDecrypt == "" || ivToDecrypt == "" {
		return nil, nil, errors.New("could not find permission in access list")
	}

	keyBytes, err := base64.URLEncoding.DecodeString(keyToDecrypt)
	if err != nil {
		return nil, nil, err
	}

	ivBytes, err := base64.URLEncoding.DecodeString(ivToDecrypt)
	if err != nil {
		return nil, nil, err
	}

	aesKey, err = rsa.DecryptOAEP(sha256.New(), rand.Reader, r.privKey, keyBytes, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to decrypt aes key. Error: %v", err)
	}
	aesIV, err = rsa.DecryptOAEP(sha256.New(), rand.Reader, r.privKey, ivBytes, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to decrypt aes IV. Error: %v", err)
	}
	return aesKey, aesIV, nil
}

func (r *Renter) RenameFile(fileId string, name string) (*core.File, error) {
	file, err := r.GetFile(fileId)
	if err != nil {
		return nil, err
	}
	if _, err := r.GetFileByName(name); err == nil {
		return nil, fmt.Errorf("%s already exists.", name)
	}

	err = r.authorizeMeta()
	if err != nil {
		return nil, err
	}

	// If it's a folder, rename it's children.
	if file.IsDir {
		children := r.findChildren(file)
		for _, child := range children {
			suffix := strings.TrimPrefix(child.Name, file.Name)
			child.Name = name + suffix
		}

		// TODO(kincaid): We need to make these renames atomic.
		// Perhaps move the rename logic to the metaserver to ensure consistency.
		for _, child := range children {
			err := r.metaClient.UpdateFile(r.Config.RenterId, child)
			if err != nil {
				r.logger.Println("RenameFile: Error updating child's name with metaserver:", err)
				return nil, err
			}
		}
	}

	file.Name = name
	err = r.metaClient.UpdateFile(r.Config.RenterId, file)
	if err != nil {
		return nil, err
	}

	err = r.saveSnapshot()
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (r *Renter) RemoveFile(fileId string, versionNum *int, recursive bool) error {
	file, err := r.GetFile(fileId)
	if err != nil {
		return err
	}
	if file.IsDir && versionNum != nil {
		return errors.New("Cannot give versionNum when removing a folder")
	}
	if versionNum != nil && len(file.Versions) == 1 {
		return errors.New("Cannot remove the only version of a file")
	}
	err = r.authorizeMeta()
	if err != nil {
		return err
	}
	if file.IsDir {
		return r.removeDir(file, recursive)
	}
	if versionNum != nil {
		return r.removeFileVersion(file, *versionNum)
	}
	return r.removeFile(file)
}

func (r *Renter) RemoveSharedFile(fileId string) error {
	err := r.authorizeMeta()
	if err != nil {
		return err
	}

	err = r.metaClient.RemoveSharedFile(r.Config.RenterId, fileId)
	if err != nil {
		return err
	}

	return nil
}

func (r *Renter) removeDir(dir *core.File, recursive bool) error {
	children := r.findChildren(dir)
	if len(children) > 0 && !recursive {
		return errors.New("Cannot remove non-empty folder without recursive option")

	}
	// Delete the file metadata. This will delete the children's metadata as well.
	err := r.metaClient.DeleteFile(r.Config.RenterId, dir.ID)
	if err != nil {
		return fmt.Errorf("Unable to delete folder metadata. Error: %s", err)
	}
	blocks := map[string][]*core.Block{}
	for _, child := range children {
		if !child.IsDir {
			for i := 0; i < len(child.Versions); i++ {
				version := &child.Versions[i]
				for j := 0; j < len(version.Blocks); j++ {
					block := &version.Blocks[j]
					blocks[block.Location.Addr] = append(blocks[block.Location.Addr], block)
				}
			}
		}
	}
	for addr, blockList := range blocks {
		pvdr := provider.NewClient(addr, netClient)
		err := pvdr.AuthorizeRenter(r.privKey, r.Config.RenterId)
		if err != nil {
			r.logger.Println("removeDir: error authorizing renter. error:", err)
			// Continue regardless
		}
		for _, block := range blockList {
			r.removeBlockFrom(block, pvdr)
		}
	}
	// Update the local file cache
	err = r.pullFiles()
	if err != nil {
		r.logger.Println("Unable to pull updates files from metaserver after removing folder.")
		r.logger.Println("Error: ", err)
	}
	return nil
}

func (r *Renter) removeFileVersion(file *core.File, versionNum int) error {
	var version *core.Version
	var versionIdx int
	for ; versionIdx < len(file.Versions); versionIdx++ {
		if file.Versions[versionIdx].Num == versionNum {
			version = &file.Versions[versionIdx]
			break
		}
	}
	if version == nil {
		return fmt.Errorf("Cannot find version %d", versionNum)
	}
	err := r.metaClient.DeleteFileVersion(r.Config.RenterId, file.ID, versionNum)
	if err != nil {
		return fmt.Errorf("Unable to delete version metadata. Error: %s", err)
	}
	r.removeVersionContents(version)
	file.Versions = append(file.Versions[:versionIdx], file.Versions[versionIdx+1:]...)
	err = r.saveSnapshot()
	if err != nil {
		r.logger.Println("Error saving snapshot:", err)
	}
	return nil
}

func (r *Renter) removeFile(file *core.File) error {
	err := r.metaClient.DeleteFile(r.Config.RenterId, file.ID)
	if err != nil {
		return err
	}
	r.removeFileContents(file)

	// Delete locally
	r.mu.Lock()
	defer r.mu.Unlock()
	for idx, file2 := range r.files {
		if file2.ID == file.ID {
			r.files = append(r.files[:idx], r.files[idx+1:]...)
			err = r.saveSnapshot()
			if err != nil {
				r.logger.Println("Error saving snapshot:", err)
			}
			break
		}
	}

	return nil
}

func (r *Renter) removeFileContents(file *core.File) {
	for i := 0; i < len(file.Versions); i++ {
		r.removeVersionContents(&file.Versions[i])
	}
}

// Deletes the blocks for a file version from the providers
// where they are stored, reclaiming the freed storage space.
func (r *Renter) removeVersionContents(version *core.Version) {
	for _, block := range version.Blocks {
		r.removeBlock(&block)
	}
}

// Removes a block from the provider where it is stored, reclaiming
// the block's storage for the freelist.
func (r *Renter) removeBlock(block *core.Block) {
	pvdr := provider.NewClient(block.Location.Addr, netClient)
	err := pvdr.AuthorizeRenter(r.privKey, r.Config.RenterId)
	if err == nil {
		err = pvdr.RemoveBlock(r.Config.RenterId, block.ID)
	}
	if err != nil {
		r.logger.Printf("Unable to remove block %s from provider %s\n",
			block.ID, block.Location.ProviderId)
		r.logger.Printf("Error: %s\n", err)
		r.blocksToDelete = append(r.blocksToDelete, block)
		return
	}

	// Reclaim the storage used by the block.
	blob := &storageBlob{
		ProviderId: block.Location.ProviderId,
		Addr:       block.Location.Addr,
		Amount:     block.Size,
		ContractId: block.Location.ContractId,
	}
	r.storageManager.AddBlob(blob)
}

func (r *Renter) removeBlockFrom(block *core.Block, pvdr *provider.Client) {
	err := pvdr.RemoveBlock(r.Config.RenterId, block.ID)
	if err != nil {
		r.logger.Printf("Unable to remove block %s from provider %s\n",
			block.ID, block.Location.ProviderId)
		r.logger.Printf("Error: %s\n", err)
		r.blocksToDelete = append(r.blocksToDelete, block)
		return
	}
	blob := &storageBlob{
		ProviderId: block.Location.ProviderId,
		Addr:       block.Location.Addr,
		Amount:     block.Size,
		ContractId: block.Location.ContractId,
	}
	r.storageManager.AddBlob(blob)
}

func (r *Renter) CreatePaypalPayment(amount int64, returnURL, cancelURL string) (string, error) {
	err := r.authorizeMeta()
	if err != nil {
		return "", err
	}

	id, err := r.metaClient.CreatePaypalPayment(amount, returnURL, cancelURL)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (r *Renter) ExecutePaypalPayment(paymentID, payerID string) error {
	err := r.authorizeMeta()
	if err != nil {
		return err
	}

	err = r.metaClient.ExecutePaypalPayment(paymentID, payerID, r.Config.RenterId)
	if err != nil {
		return err
	}

	return nil
}

func (r *Renter) Withdraw(email string, amount int64) error {
	err := r.authorizeMeta()
	if err != nil {
		return err
	}

	err = r.metaClient.RenterWithdraw(r.Config.RenterId, email, amount)
	if err != nil {
		return err
	}

	return nil
}

func (r *Renter) ListTransactions() ([]core.Transaction, error) {
	err := r.authorizeMeta()
	if err != nil {
		return nil, err
	}

	transactions, err := r.metaClient.GetRenterTransactions(r.Config.RenterId)
	if err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *Renter) ListContracts() ([]*core.Contract, error) {
	err := r.authorizeMeta()
	if err != nil {
		return nil, err
	}
	contracts, err := r.metaClient.GetRenterContracts(r.Config.RenterId)
	if err != nil {
		return nil, err
	}
	return contracts, nil
}

func (r *Renter) updateFileVersion(fileId string, versionNum int, newVersion *core.Version) error {
	err := r.authorizeMeta()
	if err != nil {
		return err
	}
	file, err := r.GetFile(fileId)
	if err != nil {
		return err
	}
	for idx := 0; idx < len(file.Versions); idx++ {
		if file.Versions[idx].Num == versionNum {
			oldCpy := file.Versions[idx]
			file.Versions[idx] = *newVersion
			err = r.metaClient.UpdateFile(r.Config.RenterId, file)
			if err != nil {
				file.Versions[idx] = oldCpy
				return err
			}
			return nil
		}
	}
	return errors.New("version does not exist")
}

func (r *Renter) saveFile(f *core.File) error {
	err := r.authorizeMeta()
	if err != nil {
		return err
	}
	err = r.metaClient.PostFile(r.Config.RenterId, f)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.files = append(r.files, f)
	r.mu.Unlock()
	return r.saveSnapshot()
}

func (r *Renter) findChildren(dir *core.File) []*core.File {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var children []*core.File
	for _, f := range r.files {
		if f != dir && strings.HasPrefix(f.Name, dir.Name) &&
			len(f.Name) > len(dir.Name) &&
			f.Name[len(dir.Name)] == '/' {

			children = append(children, f)
		}
	}
	return children
}

func (r *Renter) GetFileByName(name string) (*core.File, error) {
	files, err := r.ListFiles()
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.Name == name {
			return file, nil
		}
	}
	return nil, fmt.Errorf("Cannot find file %s", name)
}

// Authorize with the metaclient, if necessary
func (r *Renter) authorizeMeta() error {
	if !r.metaClient.IsAuthorized() {
		err := r.metaClient.AuthorizeRenter(r.privKey, r.Config.RenterId)
		if err != nil {
			return fmt.Errorf("Unable to authorize with metaserver. Error: %s", err)
		}
	}
	return nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return util.UnmarshalPrivateKey(data)
}
