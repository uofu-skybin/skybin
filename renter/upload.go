package renter

import (
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/klauspost/reedsolomon"
	"io"
	"io/ioutil"
	mathrand "math/rand"
	"net/http"
	"os"
	"path/filepath"
	"skybin/core"
	"skybin/provider"
	"skybin/util"
	"strings"
	"sync"
	"time"
)

// fileUpload stores the state for an upload as it passes through
// the stages of the upload pipeline.
type fileUpload struct {

	// Initial input
	sourcePath string
	finfo      os.FileInfo
	destPath   string

	// Compression phase result
	cTemp *os.File // Temp for compressed file data

	// Encryption phase result
	aesKey []byte
	aesIV  []byte
	eTemp  *os.File // Temp for encrypted data

	// Erasure coding phase result
	blockSize       int64
	numDataBlocks   int
	numParityBlocks int
	paddingBytes    int64
	parityFiles     []*os.File

	// Metadata phase result
	blocks  []core.Block
	version *core.Version
}

func (up *fileUpload) cleanup() {
	if up.cTemp != nil {
		up.cTemp.Close()
		os.Remove(up.cTemp.Name())
	}
	if up.eTemp != nil {
		up.eTemp.Close()
		os.Remove(up.eTemp.Name())
	}
	for _, f := range up.parityFiles {
		f.Close()
		os.Remove(f.Name())
	}
}

type folderUpload struct {
	destPath string
	isRoot   bool
}

type blockUpload struct {
	block *core.Block
	blob  *storageBlob
	data  io.Reader
	done  bool
	err   error
}

func (r *Renter) Upload(sourcePath string, destPath string, shouldOverwrite bool) (*core.File, error) {
	destPath = util.CleanPath(destPath)
	finfo, err := os.Stat(sourcePath)
	if err != nil {
		return nil, err
	}
	existingFile := r.getFileByName(destPath)
	if existingFile != nil && finfo.IsDir() {
		return nil, errors.New("A file with that name already exists.")
	}
	err = r.authorizeMeta()
	if err != nil {
		return nil, err
	}
	if finfo.IsDir() {
		return r.uploadDir(sourcePath, destPath)
	}
	if r.storageAvailable() <= finfo.Size() {
		return nil, errors.New("Not enough storage")
	}
	if existingFile != nil {
		return r.uploadVersion(sourcePath, finfo, existingFile, shouldOverwrite)
	}
	return r.uploadFile(sourcePath, finfo, destPath)
}

// Upload a new version of an existing file from sourcePath.
// finfo should be the file info for the source file. shouldOverwrite
// gives whether to overwrite the latest version of the existing file
// or create a new version.
func (r *Renter) uploadVersion(sourcePath string, finfo os.FileInfo,
	existingFile *core.File, shouldOverwrite bool) (*core.File, error) {
	aesKey, aesIV, err := r.decryptEncryptionKeys(existingFile)
	if err != nil {
		return nil, err
	}
	up := fileUpload{
		sourcePath: sourcePath,
		finfo:      finfo,
		destPath:   existingFile.Name,
		aesKey:     aesKey,
		aesIV:      aesIV,
	}
	defer up.cleanup()
	err = r.doUploads([]*fileUpload{&up})
	if err != nil {
		return nil, err
	}
	newVersion := up.version
	if newVersion == nil {
		panic("uploadVersion: doUploads() did not create version metadata")
	}

	// TODO(Kincaid): These interactions with the metaserver are error prone.
	// Consider a single convenience endpoint to overwrite the latest version
	// of a file which returns an updated copy of the file object.
	err = r.metaClient.PostFileVersion(r.Config.RenterId, existingFile.ID, newVersion)
	if err != nil {
		r.removeVersionContents(newVersion)
		return nil, fmt.Errorf("Unable to update version metadata. Error: %s", err)
	}
	if shouldOverwrite {
		prevVersion := &existingFile.Versions[len(existingFile.Versions)-1]
		err = r.metaClient.DeleteFileVersion(r.Config.RenterId, existingFile.ID, prevVersion.Num)
		if err != nil {
			r.logger.Println("Unable to overwrite previous file version. Error:", err)
		} else {
			r.removeVersionContents(prevVersion)
			existingFile.Versions = existingFile.Versions[:len(existingFile.Versions)-1]
		}
	}

	// Pull down the file again to refresh the version information.
	updatedFile, err := r.metaClient.GetFile(r.Config.RenterId, existingFile.ID)
	if err != nil {
		r.logger.Println("Unable to pull updated version of file. Error: %s", err)
		existingFile.Versions = append(existingFile.Versions, *newVersion)
		return existingFile, nil
	}
	*existingFile = *updatedFile
	err = r.saveSnapshot()
	if err != nil {
		r.logger.Println("Error saving snapshot:", err)
	}
	return updatedFile, nil
}

// Uploads a new file from sourcePath to destPath. File size and destPath validation
// should already have been performed. finfo should be the file info for the source file.
func (r *Renter) uploadFile(sourcePath string, finfo os.FileInfo, destPath string) (*core.File, error) {
	up := fileUpload{
		sourcePath: sourcePath,
		finfo:      finfo,
		destPath:   destPath,
	}
	defer up.cleanup()
	err := r.doUploads([]*fileUpload{&up})
	if err != nil {
		return nil, err
	}
	file, err := r.makeNewFile(&up)
	if err != nil {
		r.undoUpload(&up)
		return nil, err
	}
	err = r.saveFile(file)
	if err != nil {
		r.undoUpload(&up)
		return nil, err
	}
	return file, nil
}

// Uploads a directory from sourcePath to destPath.
// Returns the root folder of the new directory.
func (r *Renter) uploadDir(sourcePath string, destPath string) (*core.File, error) {

	var files []*fileUpload
	var folders []folderUpload
	var totalSize int64

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fullPath := destPath
		if path != sourcePath {
			relPath := strings.TrimPrefix(path, sourcePath)
			fullPath += relPath
		}
		if info.IsDir() {
			folders = append(folders, folderUpload{
				destPath: fullPath,
				isRoot:   path == sourcePath,
			})
		} else {
			totalSize += info.Size()
			up := &fileUpload{
				sourcePath: path,
				finfo:      info,
				destPath:   fullPath,
			}
			// It is extremely important that this be called
			// in order to close and remove temporary files created
			// in the upload process.
			defer up.cleanup()
			files = append(files, up)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	// Pre-check that enough storage is available to upload all files.
	if r.storageAvailable() <= totalSize {
		return nil, errors.New("Not enough storage")
	}

	return r.doUploadDir(files, folders)
}

func (r *Renter) doUploadDir(fileUploads []*fileUpload, folderUploads []folderUpload) (*core.File, error) {

	// Save folder metadata first to ensure we aren't derailed by
	// something silly like a name conflict.
	var rootFolder *core.File
	var folders []*core.File
	removeFolders := func() {
		for _, f := range folders {
			err := r.removeFile(f)
			if err != nil {
				r.logger.Println("Error removing folder during failed dir upload. Error: ", err)
			}
		}
	}
	for _, folderUp := range folderUploads {
		f, err := r.CreateFolder(folderUp.destPath)
		if err != nil {
			removeFolders()
			return nil, err
		}
		if folderUp.isRoot {
			rootFolder = f
		}
		folders = append(folders, f)
	}

	// Upload the files to providers.
	// This is where the real work happens.
	err := r.doUploads(fileUploads)
	if err != nil {
		removeFolders()
		return nil, err
	}
	undoUploads := func() {
		for _, up := range fileUploads {
			r.undoUpload(up)
		}
	}

	// Create file metadata
	var files []*core.File
	for _, up := range fileUploads {
		f, err := r.makeNewFile(up)
		if err != nil {
			removeFolders()
			undoUploads()
			return nil, err
		}
		files = append(files, f)
	}

	// Save file metadata
	var savedFiles []*core.File
	removeFiles := func() {
		for _, f := range savedFiles {
			err := r.removeFile(f)
			if err != nil {
				r.logger.Println("Error removing file during dir upload failure. Error: ", err)
			}
		}
	}
	for _, file := range files {
		err := r.saveFile(file)
		if err != nil {
			removeFolders()
			removeFiles()
			return nil, err
		}
		savedFiles = append(savedFiles, file)
	}

	return rootFolder, nil
}

// Prepares and performs a collection of uploads, including preparing source files,
// finding storage, and uploading file blocks to providers. This routine is all or
// nothing; either all blocks of all files are successfully uploaded, or none are.
// Does not save file metadata.
func (r *Renter) doUploads(fileUploads []*fileUpload) error {
	type prepTask struct {
		up  *fileUpload
		err error
	}

	var prepTasks []*prepTask
	doneTasks := make(chan *prepTask)
	for _, up := range fileUploads {
		prepTasks = append(prepTasks, &prepTask{up: up})
	}
	for _, t := range prepTasks {
		go func(t *prepTask) {
			t.err = prepareUpload(t.up, r.Config)
			doneTasks <- t
		}(t)
	}

	// Queues for block uploads indexed by provider ID
	uploadQueues := map[string]chan *blockUpload{}
	var allBlockUploads []*blockUpload
	var err error
	var wg sync.WaitGroup

	for _, _ = range fileUploads {
		t := <-doneTasks
		if t.err != nil {
			r.logger.Printf("Error preparing file %s for upload. Error: %s\n",
				t.up.sourcePath, t.err)
			if err == nil {
				err = t.err
			}
		}
		if err != nil {
			continue
		}
		var blockUploads []*blockUpload
		blockUploads, err = r.findUploadStorage(t.up)
		if err != nil {
			r.logger.Printf("Unable to find enough storage for file %s. Error: %s\n",
				t.up.sourcePath, err)
			continue
		}

		// Save the uploads for this file to our list for all files.
		allBlockUploads = append(allBlockUploads, blockUploads...)

		// Upload the blocks for this file by funnelling them to
		// the appropriate upload worker thread.
		for _, blockUp := range blockUploads {
			q, exists := uploadQueues[blockUp.blob.ProviderId]
			if !exists {
				q = make(chan *blockUpload, 16)
				uploadQueues[blockUp.blob.ProviderId] = q
				wg.Add(1)
				go func(providerAddr string) {
					defer wg.Done()
					r.uploadWorker(providerAddr, q)
				}(blockUp.blob.Addr)
			}
			q <- blockUp
		}

	}
	// Shut down upload workers and wait for them to finish
	for _, q := range uploadQueues {
		close(q)
	}
	wg.Wait()

	freeBlobs := func() {
		for _, bUp := range allBlockUploads {
			if bUp.done {
				r.removeBlock(bUp.block)
			}
		}
	}

	// Check if a an error occurred during a prep task.
	if err != nil {
		freeBlobs()
		return err
	}

	// Check if an error occurred during a block upload.
	for _, bUp := range allBlockUploads {
		if bUp.err != nil {
			freeBlobs()
			return bUp.err
		}
	}
	return nil
}

func (r *Renter) uploadWorker(providerAddr string, blockQ chan *blockUpload) {
	client := provider.NewClient(providerAddr, &http.Client{})
	err := client.AuthorizeRenter(r.privKey, r.Config.RenterId)
	for upload := range blockQ {
		if err != nil {
			upload.err = err
			continue
		}
		if upload.block.Location.Addr != providerAddr {
			msg := "uploadWorker: received block for wrong provider"
			r.logger.Println(msg)
			panic(msg)
		}
		err = client.PutBlock(r.Config.RenterId, upload.block.ID, upload.data)
		if err != nil {
			upload.err = err
			continue
		}
		upload.done = true
	}
}

// This is the final preparation phase for an upload before transferring
// the source blocks to storage providers. It finds storage for the
// upload's blocks, updates the block metadata, and prepares the blockUpload
// structures necessary to complete the uploads.
func (r *Renter) findUploadStorage(up *fileUpload) ([]*blockUpload, error) {
	totalBlocks := up.numDataBlocks + up.numParityBlocks
	blobs, err := r.findStorage(totalBlocks, up.blockSize)
	if err != nil {
		return nil, err
	}

	// Update block locations
	for i := 0; i < len(up.blocks); i++ {
		up.blocks[i].Location = core.BlockLocation{
			ProviderId: blobs[i].ProviderId,
			Addr:       blobs[i].Addr,
			ContractId: blobs[i].ContractId,
		}
	}

	blockReaders := []io.Reader{}
	for blockNum := 0; blockNum < up.numDataBlocks; blockNum++ {
		r := io.NewSectionReader(up.eTemp, up.blockSize*int64(blockNum), up.blockSize)
		blockReaders = append(blockReaders, r)
	}
	for _, parityFile := range up.parityFiles {
		r := io.NewSectionReader(parityFile, 0, up.blockSize)
		blockReaders = append(blockReaders, r)
	}

	var blockUploads []*blockUpload
	for i := 0; i < len(up.blocks); i++ {
		blockUploads = append(blockUploads, &blockUpload{
			block: &up.blocks[i],
			blob:  blobs[i],
			data:  blockReaders[i],
			done:  false,
		})
	}

	return blockUploads, nil
}

func (r *Renter) undoUpload(up *fileUpload) {
	r.removeVersionContents(up.version)
}

// Creates file metadata from upload metadata.
func (r *Renter) makeNewFile(up *fileUpload) (*core.File, error) {
	fileId, err := genId()
	if err != nil {
		return nil, err
	}
	aesKeyEncrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &r.privKey.PublicKey, up.aesKey, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to encrypt aes key. Error: %s", err)
	}
	aesIVEncrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &r.privKey.PublicKey, up.aesIV, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to encrypt aes IV. Error: %s", err)
	}
	versions := []core.Version{
		*up.version,
	}
	file := &core.File{
		ID:         fileId,
		OwnerID:    r.Config.RenterId,
		Name:       up.destPath,
		IsDir:      false,
		AccessList: []core.Permission{},
		AesKey:     base64.URLEncoding.EncodeToString(aesKeyEncrypted),
		AesIV:      base64.URLEncoding.EncodeToString(aesIVEncrypted),
		Versions:   versions,
	}
	return file, nil
}

// Prepares a source file for upload. This will fill out the given fileUpload
// struct as it completes stages of the upload preparation pipeline.
// Note that this does not include finding storage for the upload.
func prepareUpload(up *fileUpload, conf *Config) error {
	err := compress(up)
	if err != nil {
		return err
	}

	if up.aesKey == nil {
		err = createKeys(up)
		if err != nil {
			return err
		}
	}

	err = encrypt(up)
	if err != nil {
		return err
	}

	err = erasureCode(up, conf)
	if err != nil {
		return err
	}

	err = prepareMetadata(up)
	if err != nil {
		return err
	}

	return nil
}

// Preparation phase 1: encrypt the source file.
func compress(up *fileUpload) error {
	cTemp, err := ioutil.TempFile("", "skybin_upload")
	if err != nil {
		return fmt.Errorf("Unable to create temp file. Error: %s", err)
	}
	srcFile, err := os.Open(up.sourcePath)
	if err != nil {
		return fmt.Errorf("Unable to open source file. Error: %s", err)
	}
	defer srcFile.Close()
	cw := zlib.NewWriter(cTemp)
	_, err = io.Copy(cw, srcFile)
	if err != nil {
		return fmt.Errorf("Compression error. Error: %s", err)
	}
	cw.Close()

	up.cTemp = cTemp

	return nil
}

func createKeys(up *fileUpload) error {
	aesKey := make([]byte, 32)
	_, err := rand.Reader.Read(aesKey)
	if err != nil {
		return fmt.Errorf("Unable to create encryption key. Error: %s", err)
	}
	aesIV := make([]byte, aes.BlockSize)
	_, err = rand.Reader.Read(aesIV)
	if err != nil {
		return fmt.Errorf("Unable to read initialization vector. Error: %s", err)
	}

	up.aesKey = aesKey
	up.aesIV = aesIV

	return nil
}

// Preparation phase 2: encrypt the compressed file.
func encrypt(up *fileUpload) error {
	eTemp, err := ioutil.TempFile("", "skybin_upload")
	if err != nil {
		return fmt.Errorf("Unable to create temp file. Error: %s", err)
	}
	aesCipher, err := aes.NewCipher(up.aesKey)
	if err != nil {
		return fmt.Errorf("Unable to create encryption cipher. Error: %s", err)
	}
	streamWriter := cipher.StreamWriter{
		S: cipher.NewCFBEncrypter(aesCipher, up.aesIV),
		W: eTemp,
	}
	_, err = up.cTemp.Seek(0, os.SEEK_SET)
	if err != nil {
		return fmt.Errorf("Unable to seek file. Error: %s", err)
	}
	_, err = io.Copy(streamWriter, up.cTemp)
	if err != nil {
		return fmt.Errorf("Unable to encrypt file. Error: %s", err)
	}

	up.eTemp = eTemp

	return nil
}

// Preparation phase 3: determine block sizes and compute erasure codes.
// This phase creates additional temp files for parity blocks.
// It may also truncate up.eTemp to the next block size multiple to
// meet an alignment requirement for erasure coding.
func erasureCode(up *fileUpload, conf *Config) error {
	st, err := up.eTemp.Stat()
	if err != nil {
		return fmt.Errorf("Unable to stat temp file. Error: %s", err)
	}
	blockSize := (st.Size() + int64(conf.DefaultDataBlocks) - 1) / int64(conf.DefaultDataBlocks)
	if blockSize > conf.MaxBlockSize {
		blockSize = conf.MaxBlockSize
	}
	if blockSize > conf.MaxContractSize {
		blockSize = conf.MaxContractSize
	}
	// Pad file up to next nearest block size multiple if necessary.
	// Its size must be a block size multiple.
	paddingBytes := (blockSize - (st.Size() % blockSize)) % blockSize
	if paddingBytes != 0 {
		err := up.eTemp.Truncate(st.Size() + paddingBytes)
		if err != nil {
			return fmt.Errorf("Unable to pad file. Error: %s", err)
		}
	}
	nDataBlocks := int((st.Size() + blockSize - 1) / blockSize)
	nParityBlocks := conf.DefaultParityBlocks
	if nDataBlocks > conf.DefaultDataBlocks {
		nParityBlocks = (nDataBlocks + 1) / 2
	}
	encoder, err := reedsolomon.NewStream(nDataBlocks, nParityBlocks)
	if err != nil {
		return fmt.Errorf("Unable to create erasure encoder. Error: %s", err)
	}
	// Create parity files
	var parityFiles []*os.File
	for i := 0; i < nParityBlocks; i++ {
		f, err := ioutil.TempFile("", "skybin_upload")
		if err != nil {
			return fmt.Errorf("Unable to create parity file. Error: %s", err)
		}
		parityFiles = append(parityFiles, f)
	}
	var blockReaders []io.Reader
	for i := 0; i < nDataBlocks; i++ {
		blockReaders = append(blockReaders, io.NewSectionReader(up.eTemp, blockSize*int64(i), blockSize))
	}
	err = encoder.Encode(blockReaders, convertToWriterSlice(parityFiles))
	if err != nil {
		return fmt.Errorf("Unable to create erasure codes. Error: %s", err)
	}

	up.blockSize = blockSize
	up.numDataBlocks = nDataBlocks
	up.numParityBlocks = nParityBlocks
	up.paddingBytes = paddingBytes
	up.parityFiles = parityFiles

	return nil
}

// Preparation phase 4: create block and version metadata.
func prepareMetadata(up *fileUpload) error {
	blockReaders := []io.Reader{}
	for blockNum := 0; blockNum < up.numDataBlocks; blockNum++ {
		blockReaders = append(blockReaders, io.NewSectionReader(up.eTemp, up.blockSize*int64(blockNum), up.blockSize))
	}
	for i := 0; i < up.numParityBlocks; i++ {
		_, err := up.parityFiles[i].Seek(0, os.SEEK_SET)
		if err != nil {
			return fmt.Errorf("Unable to seek parity file. Error: %s", err)
		}
		blockReaders = append(blockReaders, up.parityFiles[i])
	}

	// Generate block metadata
	var blocks []core.Block
	for blockNum, blockReader := range blockReaders {
		blockId, err := genId()
		if err != nil {
			return fmt.Errorf("Unable to create block ID. Error: %s", err)
		}
		h := sha256.New()
		n, err := io.Copy(h, blockReader)
		if err != nil {
			return fmt.Errorf("Unable to calculate block hash. Error: %s", err)
		}
		blockHash := base64.URLEncoding.EncodeToString(h.Sum(nil))
		block := core.Block{
			ID:         blockId,
			Num:        blockNum,
			Size:       n,
			Sha256Hash: blockHash,
		}
		blocks = append(blocks, block)
	}

	// Generate version metadata
	uploadSize := up.blockSize * int64(up.numDataBlocks)
	for _, f := range up.parityFiles {
		st, err := f.Stat()
		if err != nil {
			return fmt.Errorf("Unable to stat parity file. Error: %s", err)
		}
		uploadSize += st.Size()
	}
	version := &core.Version{
		ModTime:         up.finfo.ModTime(),
		Size:            up.finfo.Size(),
		UploadTime:      time.Now(),
		UploadSize:      uploadSize,
		PaddingBytes:    up.paddingBytes,
		NumDataBlocks:   up.numDataBlocks,
		NumParityBlocks: up.numParityBlocks,
		Blocks:          blocks,
	}

	up.blocks = blocks
	up.version = version

	return nil
}

func (r *Renter) findStorage(nblocks int, blockSize int64) ([]*storageBlob, error) {
	type candidate struct {
		*storageBlob
		idx int // Index of the blob in the freelist
	}

	var candidates []candidate
	for idx, blob := range r.freelist {
		if blob.Amount >= blockSize {
			candidates = append(candidates,
				candidate{storageBlob: blob, idx: idx})
		}
	}

	// Randomize the order of candidate inspection
	for i := len(candidates) - 1; i >= 0; i-- {
		j := mathrand.Intn(i + 1)
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}

	var blobs []*storageBlob
	for i := 0; len(blobs) < nblocks && len(candidates) > 0; {
		candidate := candidates[i]

		// Check if the provider is online
		client := provider.NewClient(candidate.Addr, &http.Client{})
		_, err := client.GetInfo()
		if err != nil {
			candidates = append(candidates[:i], candidates[i+1:]...)
			continue
		}

		blob := &storageBlob{
			ProviderId: candidate.ProviderId,
			Amount:     blockSize,
			Addr:       candidate.Addr,
			ContractId: candidate.ContractId,
		}
		blobs = append(blobs, blob)

		candidate.Amount -= blob.Amount
		if candidate.Amount < blockSize {
			candidates = append(candidates[:i], candidates[i+1:]...)
		}
		if candidate.Amount < kMinBlobSize {
			r.freelist = append(r.freelist[:candidate.idx], r.freelist[candidate.idx+1:]...)
		}

		i = (i + 1) % len(candidates)
	}
	if len(blobs) < nblocks {
		for _, blob := range blobs {
			r.addBlob(blob)
		}
		return nil, errors.New("Cannot find enough storage.")
	}
	return blobs, nil
}
