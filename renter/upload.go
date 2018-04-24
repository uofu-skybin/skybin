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
	"net/http"
	"os"
	"path/filepath"
	"skybin/core"
	"skybin/provider"
	"skybin/util"
	"time"
	"hash"
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

	// Closed when the upload is complete
	doneCh chan struct{}
	err    error
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

type blockUpload struct {
	block *core.Block
	blob  *storageBlob

	// Data to upload
	data   io.ReaderAt
	offset int64
	size   int64

	// Closed when the upload is complete
	doneCh chan struct{}
	err    error
}

func (bu *blockUpload) reader() io.Reader {
	return io.NewSectionReader(bu.data, bu.offset, bu.size)
}

const (

	// We limit the number of concurrent uploads to ensure
	// we don't open too many temp files at once and to
	// maximize throughput for large folder uploads.
	maxConcurrentUploads = 12
)

type folderUpload struct {
	destPath string
	isRoot   bool
}

// Uploads a new file or folder from source path to dest path. shouldOverwrite
// dictates whether an existing file with the same name should be overwritten
// or whether a new version should be added.
func (r *Renter) Upload(sourcePath string, destPath string, shouldOverwrite bool) (*core.File, error) {
	destPath = util.CleanPath(destPath)
	finfo, err := os.Stat(sourcePath)
	if err != nil {
		return nil, err
	}
	existingFile, err := r.GetFileByName(destPath)
	if err == nil && finfo.IsDir() {
		return nil, errors.New("A file with that name already exists.")
	}
	err = r.authorizeMeta()
	if err != nil {
		return nil, err
	}
	if finfo.IsDir() {
		return r.uploadDir(sourcePath, destPath)
	}
	if r.storageManager.AvailableStorage() <= finfo.Size() {
		return nil, errors.New("Not enough storage")
	}
	if existingFile != nil {
		return r.uploadVersion(sourcePath, finfo, existingFile, shouldOverwrite)
	}
	return r.uploadFile(sourcePath, finfo, destPath)
}

// Uploads a new version of an existing file from sourcePath.
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
		doneCh:     make(chan struct{}),
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
		doneCh:     make(chan struct{}),
	}
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
			relPath, err := filepath.Rel(sourcePath, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)
			fullPath += "/" + relPath
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
				doneCh:     make(chan struct{}),
			}
			files = append(files, up)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	// Pre-check that enough storage is available to upload all files.
	if r.storageManager.AvailableStorage() <= totalSize {
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
				r.logger.Println("Error removing file during dir upload failure. Error:", err)
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

func (r *Renter) doUploads(uploads []*fileUpload) error {
	for _, upload := range uploads {
		r.uploadQ <- upload
	}
	for _, upload := range uploads {
		<-upload.doneCh
	}
	failedUploads := []*fileUpload{}
	successfulUploads := []*fileUpload{}
	for _, upload := range uploads {
		if upload.err != nil {
			r.logger.Printf("Error performing upload to destpath %s: %s\n",
				upload.destPath, upload.err)
			failedUploads = append(failedUploads, upload)
		} else {
			successfulUploads = append(successfulUploads, upload)
		}
	}
	if len(failedUploads) > 0 {
		for _, upload := range successfulUploads {
			r.undoUpload(upload)
		}
		return failedUploads[0].err
	}
	return nil
}

var (
	resetProviderConns *blockUpload = nil
)

// Main upload thread for the renter. Takes all upload requests, performs
// them, and notifies the waiting request thread when the upload is complete.
// Individual blocks are uploaded in a dedicated per-provider goroutine,
// and a block schedule goroutine multiplexes the block uploads to these goroutines.
func (r *Renter) uploadThread() {
	inputQ := make(chan *fileUpload)
	finishedUploads := make(chan *fileUpload)
	blockQ := make(chan *blockUpload)
	activeUploads := 0

	for i := 0; i < maxConcurrentUploads; i++ {
		go r.fileUploadWorker(inputQ, finishedUploads, blockQ)
	}
	go r.blockUploadScheduleThread(blockQ)

L:
	for {
		select {
		case upload, ok := <-r.uploadQ:
			if !ok {
				break L
			}
			inputQ <- upload
			activeUploads++
		case <-finishedUploads:
			activeUploads--
			if activeUploads == 0 {
				select {
				case upload, ok := <-r.uploadQ:
					if !ok {
						break L
					}
					inputQ <- upload
					activeUploads++
				default:

					// Shut down open connections with providers
					blockQ <- resetProviderConns
				}
			}
		}
		if activeUploads >= maxConcurrentUploads {
			<-finishedUploads
			activeUploads--
		}
	}
	for activeUploads > 0 {
		<-finishedUploads
		activeUploads--
	}
	close(inputQ)
	close(blockQ)
}

func (r *Renter) fileUploadWorker(inputQ, finishedUploads chan *fileUpload, blockQ chan *blockUpload) {
	for upload := range inputQ {
		r.performUpload(upload, blockQ)
		finishedUploads <- upload
	}
}

func (r *Renter) performUpload(upload *fileUpload, blockQ chan *blockUpload) {
	err := prepareUpload(upload, r.Config)
	if err != nil {
		upload.err = err
		return
	}

	pendingUploads := []*blockUpload{}
	for blockNum := 0; blockNum < len(upload.blocks); blockNum++ {
		block := &upload.blocks[blockNum]
		bu := &blockUpload{
			block:  block,
			size:   upload.blockSize,
		}
		if blockNum < upload.numDataBlocks {
			bu.data = upload.eTemp
			bu.offset = upload.blockSize * int64(blockNum)
		} else {
			bu.data = upload.parityFiles[blockNum-upload.numDataBlocks]
			bu.offset = 0
		}
		pendingUploads = append(pendingUploads, bu)
	}

	finishedUploads := []*blockUpload{}
	blobsToReturn := []*storageBlob{}
	offlineProviders := map[string]bool{}
	for len(pendingUploads) > 0 {
		blobs, err := r.storageManager.FindStorageExclude(len(pendingUploads), upload.blockSize, offlineProviders)
		if err != nil {
			upload.err = err
			break
		}
		for i := 0; i < len(pendingUploads); i++ {
			bu := pendingUploads[i]
			blob := blobs[i]
			block := bu.block

			bu.blob = blob
			bu.err = nil
			bu.doneCh = make(chan struct{})

			block.Location = core.BlockLocation{
				ProviderId: blob.ProviderId,
				Addr:       blob.Addr,
				ContractId: blob.ContractId,
			}
		}
		successes, failures := doBlockUploads(pendingUploads, blockQ)
		finishedUploads = append(finishedUploads, successes...)
		for _, failure := range failures {
			r.logger.Printf("Error uploading block %s for file %s to provider %s: %s\n",
				failure.block.ID, upload.destPath, failure.blob.ProviderId, failure.err)
			blobsToReturn = append(blobsToReturn, failure.blob)
			offlineProviders[failure.blob.ProviderId] = true
		}
		pendingUploads = failures
	}
	if len(pendingUploads) > 0 {
		// The upload failed.
		for _, bu := range finishedUploads {
			blobsToReturn = append(blobsToReturn, bu.blob)
		}
		if upload.err == nil {
			upload.err = fmt.Errorf("Error uploading file. Failed to upload file blocks.")
		}
	}
	if len(blobsToReturn) > 0 {
		r.storageManager.AddBlobs(blobsToReturn)
	}
	upload.cleanup()
	close(upload.doneCh)
}

func doBlockUploads(blockUploads []*blockUpload, blockQ chan *blockUpload) (successes, failures []*blockUpload) {
	// Queue the blocks for upload
	for _, blockUpload := range blockUploads {
		blockQ <- blockUpload
	}
	// Wait for the uploads to complete
	for _, blockUpload := range blockUploads {
		<-blockUpload.doneCh
	}
	for _, bu := range blockUploads {
		if bu.err == nil {
			successes = append(successes, bu)
		} else {
			failures = append(failures, bu)
		}
	}
	return
}

func (r *Renter) blockUploadScheduleThread(blockQ chan *blockUpload) {
	uploadQueues := map[string]chan *blockUpload{}
	for upload := range blockQ {
		if upload == resetProviderConns {
			for _, q := range uploadQueues {
				close(q)
			}
			uploadQueues = map[string]chan *blockUpload{}
			continue
		}
		q, exists := uploadQueues[upload.blob.ProviderId]
		if !exists {
			q = make(chan *blockUpload)
			uploadQueues[upload.blob.ProviderId] = q
			go r.blockUploadThread(upload.blob.Addr, q)
		}
		q <- upload
	}
	for _, q := range uploadQueues {
		close(q)
	}
}

func (r *Renter) blockUploadThread(providerAddr string, blockQ chan *blockUpload) {
	client := provider.NewClient(providerAddr, &http.Client{})
	authErr := client.AuthorizeRenter(r.privKey, r.Config.RenterId)
	for upload := range blockQ {
		if upload.block.Location.Addr != providerAddr {
			msg := "uploadWorker: received block for wrong provider"
			r.logger.Println(msg)
			panic(msg)
		}
		if authErr != nil {
			upload.err = authErr
			close(upload.doneCh)
			continue
		}
		err := client.PutBlock(r.Config.RenterId, upload.block.ID, upload.reader(), upload.size)
		if err != nil {
			upload.err = err
		}
		close(upload.doneCh)
	}
}

func (r *Renter) undoUpload(up *fileUpload) {
	r.removeVersionContents(up.version)
}

// Creates file metadata from upload metadata.
func (r *Renter) makeNewFile(up *fileUpload) (*core.File, error) {
	fileId, err := util.GenerateID()
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
		OwnerAlias: r.Config.Alias,
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

	err = prepareMetadata(up, conf.NumBlockAudits)
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
func prepareMetadata(up *fileUpload, numAudits int) error {
	blockReaders := []io.Reader{}
	for blockNum := 0; blockNum < up.numDataBlocks; blockNum++ {
		blockReaders = append(blockReaders, io.NewSectionReader(up.eTemp, up.blockSize*int64(blockNum), up.blockSize))
	}
	for i := 0; i < up.numParityBlocks; i++ {
		blockReaders = append(blockReaders, io.NewSectionReader(up.parityFiles[i], 0, up.blockSize))
	}

	// Generate block metadata
	var blocks []core.Block
	for blockNum, blockReader := range blockReaders {
		blockId, err := util.GenerateID()
		if err != nil {
			return fmt.Errorf("Unable to create block ID. Error: %s", err)
		}
		auditHashes := []hash.Hash{}
		audits := []core.BlockAudit{}
		for i := 0; i < numAudits; i++ {
			nonceBytes, err := util.GenerateAuditNonce()
			if err != nil {
				return err
			}
			audit := core.BlockAudit{
				Nonce: base64.URLEncoding.EncodeToString(nonceBytes),
			}
			h := sha256.New()
			h.Write(nonceBytes)
			auditHashes = append(auditHashes, h)
			audits = append(audits, audit)
		}
		blockHash := sha256.New()
		writers := []io.Writer{blockHash}
		for _, h := range auditHashes {
			writers = append(writers, h)
		}
		// Generate the hashes from the block
		n, err := io.Copy(io.MultiWriter(writers...), blockReader)
		if err != nil {
			return fmt.Errorf("Unable to calculate block hash. Error: %s", err)
		}
		for idx, auditHash := range auditHashes {
			audit := &audits[idx]
			auditHashBytes := auditHash.Sum(nil)
			audit.ExpectedHash = base64.URLEncoding.EncodeToString(auditHashBytes)
		}
		blockHashStr := base64.URLEncoding.EncodeToString(blockHash.Sum(nil))
		block := core.Block{
			ID:         blockId,
			Num:        blockNum,
			Size:       n,
			Sha256Hash: blockHashStr,
			Audits:     audits,
			AuditPassed: true,
		}
		blocks = append(blocks, block)
	}

	// Generate version metadata
	uploadSize := up.blockSize * (int64(up.numDataBlocks) + int64(up.numParityBlocks))
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
