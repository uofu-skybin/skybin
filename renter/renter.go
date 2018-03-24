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

	"log"
	"time"

	"github.com/satori/go.uuid"
)

type Renter struct {
	Config  *Config
	Homedir string
	privKey *rsa.PrivateKey

	// An in-memory cache of the renter's file and contract metadata.
	files     []*core.File
	contracts []*core.Contract

	// All free storage blobs available for file uploads.
	// Each storage contract should have at most one associated
	// blob in this list.
	freelist []*storageBlob

	// Metaserver client. The metaserver should always have an up-to-date
	// view of every file and contract we store locally. However, the
	// opposite is not necessarily true; we may not have an up-to-date
	// view of everything the metaserver stores on our behalf.
	metaClient *metaserver.Client

	// The last time we pulled down a list of our files from the metaserver.
	lastFilesUpdate time.Time

	// Blocks which need to be removed but could not be immediately
	// deleted because the provider storing them was offline.
	blocksToDelete []*core.Block

	logger *log.Logger
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

func LoadFromDisk(homedir string) (*Renter, error) {
	renter := &Renter{
		Homedir:        homedir,
		files:          make([]*core.File, 0),
		contracts:      make([]*core.Contract, 0),
		freelist:       make([]*storageBlob, 0),
		blocksToDelete: make([]*core.Block, 0),
		logger:         log.New(ioutil.Discard, "", log.LstdFlags),
	}

	config := &Config{}
	err := util.LoadJson(path.Join(homedir, "config.json"), config)
	if err != nil {
		return nil, err
	}
	renter.Config = config

	renter.metaClient = metaserver.NewClient(config.MetaAddr, &http.Client{})

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
		renter.blocksToDelete = s.BlocksToDelete
	}

	privKey, err := loadPrivateKey(path.Join(homedir, "renterid"))
	if err != nil {
		return nil, err
	}
	renter.privKey = privKey

	return renter, err
}

func (r *Renter) SetLogger(logger *log.Logger) {
	r.logger = logger
}

func (r *Renter) saveSnapshot() error {
	s := snapshot{
		Files:          r.files,
		Contracts:      r.contracts,
		FreeStorage:    r.freelist,
		BlocksToDelete: r.blocksToDelete,
	}
	return util.SaveJson(path.Join(r.Homedir, "snapshot.json"), &s)
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
	if r.getFileByName(name) != nil {
		return nil, fmt.Errorf("%s already exists.", name)
	}
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
	file, err := r.GetFile(fileId)
	if err != nil {
		return nil, err
	}
	if r.getFileByName(name) != nil {
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
				return nil, err
				r.logger.Println("RenameFile: Error updating child's name with metaserver:", err)
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

	// Ugly conversion of []File to []*File
	r.files = []*core.File{}
	for i := 0; i < len(files); i++ {
		r.files = append(r.files, &files[i])
	}

	r.lastFilesUpdate = time.Now()

	return r.saveSnapshot()
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

func (r *Renter) GetFile(fileId string) (*core.File, error) {

	// Check if it exists locally
	for _, file := range r.files {
		if file.ID == fileId {
			return file, nil
		}
	}

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

	r.files = append(r.files, file)

	return file, nil
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

func (r *Renter) RemoveFile(fileId string, versionNum *int) error {
	file, err := r.GetFile(fileId)
	if err != nil {
		return err
	}
	if file.IsDir && versionNum != nil {
		return errors.New("Cannot give versionNum when removing a folder")
	}
	if versionNum != nil && len(file.Versions) == 1 {
		return errors.New("Cannot remove only version of file")
	}
	if file.IsDir && len(r.findChildren(file)) > 0 {
		return errors.New("Cannot remove non-empty folder")
	}
	err = r.authorizeMeta()
	if err != nil {
		return err
	}
	if versionNum != nil {
		return r.removeFileVersion(file, *versionNum)
	}
	return r.removeFile(file)
}

func (r *Renter) removeFile(file *core.File) error {
	err := r.metaClient.DeleteFile(r.Config.RenterId, file.ID)
	if err != nil {
		return err
	}
	for _, version := range file.Versions {
		r.removeVersionBlocks(&version)
	}

	// Delete locally
	fileIdx := -1
	for idx, file2 := range r.files {
		if file2.ID == file.ID {
			fileIdx = idx
			break
		}
	}
	if fileIdx != -1 {
		r.files = append(r.files[:fileIdx], r.files[fileIdx+1:]...)
		err = r.saveSnapshot()
		if err != nil {
			r.logger.Println("Error saving snapshot:", err)
		}
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
	r.removeVersionBlocks(version)
	file.Versions = append(file.Versions[:versionIdx], file.Versions[versionIdx+1:]...)
	err = r.saveSnapshot()
	if err != nil {
		r.logger.Println("Error saving snapshot:", err)
	}
	return nil
}

// Deletes the blocks for a file version from the providers
// where they are stored, reclaiming the freed storage space.
func (r *Renter) removeVersionBlocks(version *core.Version) {
	for _, block := range version.Blocks {
		r.removeBlock(&block)
	}
}

// Removes a block from the provider where it is stored, reclaiming
// the block's storage for the freelist.
func (r *Renter) removeBlock(block *core.Block) {
	pvdr := provider.NewClient(block.Location.Addr, &http.Client{})
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
	r.addBlob(blob)
}

// Add a storage blob back to the free list.
func (r *Renter) addBlob(blob *storageBlob) {
	for _, blob2 := range r.freelist {
		if blob.ContractId == blob2.ContractId {

			// Merge blobs
			blob2.Amount += blob.Amount
			return
		}
	}
	r.freelist = append(r.freelist, blob)
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
	r.files = append(r.files, f)
	return r.saveSnapshot()
}

func (r *Renter) findChildren(dir *core.File) []*core.File {
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

func (r *Renter) getFileByName(name string) *core.File {
	for _, file := range r.files {
		if file.Name == name {
			return file
		}
	}
	return nil
}

func (r *Renter) storageAvailable() int64 {
	var total int64 = 0
	for _, blob := range r.freelist {
		total += blob.Amount
	}
	return total
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

func genId() (string, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
