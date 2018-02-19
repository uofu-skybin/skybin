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
)

func (r *Renter) Upload(srcPath string, destPath string, shouldOverwrite bool) (*core.File, error) {
	destPath = util.CleanPath(destPath)
	finfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	existingFile := r.getFileByName(destPath)
	if existingFile != nil && finfo.IsDir() {
		return nil, errors.New("A file with that name already exists.")
	}
	if finfo.IsDir() {
		return r.uploadDir(srcPath, destPath)
	}
	if r.storageAvailable() <= finfo.Size() {
		return nil, errors.New("Not enough storage")
	}
	if existingFile != nil {
		aesKey, aesIV, err := r.decryptEncryptionKeys(existingFile)
		if err != nil {
			return nil, err
		}

		// Authorize with metaserver before performing upload of new version
		// to catch authorization error early.
		err = r.authorizeMeta()
		if err != nil {
			return nil, err
		}
		newVersion, err := r.performUpload(srcPath, finfo, aesKey, aesIV)
		if err != nil {
			return nil, err
		}

		// TODO(Kincaid): These interactions with the metaserver are error prone.
		// Consider a single convenience endpoint to overwrite the latest version
		// of a file which returns an updated copy of the file object.
		err = r.metaClient.PostFileVersion(r.Config.RenterId, existingFile.ID, newVersion)
		if err != nil {
			r.removeVersionBlocks(newVersion)
			return nil, fmt.Errorf("Unable to update version metadata. Error: %s", err)
		}

		if shouldOverwrite {
			prevVersion := &existingFile.Versions[len(existingFile.Versions)-1]
			err = r.metaClient.DeleteFileVersion(r.Config.RenterId, existingFile.ID, prevVersion.Num)
			if err != nil {
				r.logger.Println("Unable to overwrite previous file version. Error:", err)
			} else {
				r.removeVersionBlocks(prevVersion)
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
	return r.uploadFile(srcPath, finfo, destPath)
}

// Uploads a new file from srcPath to destPath. File size and destPath validation
// should already have been performed. finfo should be the file info for the source file.
func (r *Renter) uploadFile(srcPath string, finfo os.FileInfo, destPath string) (*core.File, error) {
	fileId, err := genId()
	if err != nil {
		return nil, err
	}
	aesKey := make([]byte, 32)
	_, err = rand.Reader.Read(aesKey)
	if err != nil {
		return nil, fmt.Errorf("Unable to create encryption key. Error: %s", err)
	}
	aesIV := make([]byte, aes.BlockSize)
	_, err = rand.Reader.Read(aesIV)
	if err != nil {
		return nil, fmt.Errorf("Unable to read initialization vector. Error: %s", err)
	}
	aesKeyEncrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &r.privKey.PublicKey, aesKey, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to encrypt aes key. Error: %s", err)
	}
	aesIVEncrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &r.privKey.PublicKey, aesIV, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to encrypt aes IV. Error: %s", err)
	}
	version, err := r.performUpload(srcPath, finfo, aesKey, aesIV)
	if err != nil {
		return nil, err
	}
	file := &core.File{
		ID:         fileId,
		OwnerID:    r.Config.RenterId,
		Name:       destPath,
		IsDir:      false,
		AccessList: []core.Permission{},
		AesKey:     base64.URLEncoding.EncodeToString(aesKeyEncrypted),
		AesIV:      base64.URLEncoding.EncodeToString(aesIVEncrypted),
		Versions:   []core.Version{},
	}
	// TODO: Kincaid. Do I have to set this?
	version.Num = 1
	file.Versions = append(file.Versions, *version)
	err = r.saveFile(file)
	if err != nil {
		r.removeVersionBlocks(version)
		err2 := r.saveSnapshot()
		if err2 != nil {
			r.logger.Println("Error saving snapshot:", err2)
		}
	}
	return file, err
}

// Uploads a directory from srcPath to destPath. Returns the root folder of the new directory.
func (r *Renter) uploadDir(srcPath string, destPath string) (*core.File, error) {
	var size int64
	err := filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if r.storageAvailable() <= size {
		return nil, errors.New("Not enough storage")
	}
	var rootFolder *core.File
	err = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fullPath := destPath
		if path != srcPath {
			relPath := strings.TrimPrefix(path, srcPath)
			fullPath += relPath
		}
		if info.IsDir() {
			f, err := r.CreateFolder(fullPath)
			if err != nil {
				return err
			}
			if path == srcPath {
				rootFolder = f
			}
			return nil
		}
		_, err = r.uploadFile(path, info, fullPath)
		return err
	})
	return rootFolder, nil
}

func (r *Renter) performUpload(srcPath string, finfo os.FileInfo, aesKey []byte, aesIV []byte) (*core.Version, error) {

	// Compress
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("Unable to open source file. Error: %s", err)
	}
	defer srcFile.Close()
	temp1, err := ioutil.TempFile("", "skybin_upload")
	if err != nil {
		return nil, fmt.Errorf("Unable to create temp file. Error: %s", err)
	}
	defer temp1.Close()
	defer os.Remove(temp1.Name())
	cw := zlib.NewWriter(temp1)
	_, err = io.Copy(cw, srcFile)
	if err != nil {
		return nil, fmt.Errorf("Compression error. Error: %s", err)
	}
	cw.Close()

	// Encrypt
	aesCipher, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("Unable to create encryption cipher. Error: %s", err)
	}
	temp2, err := ioutil.TempFile("", "skybin_upload")
	if err != nil {
		return nil, fmt.Errorf("Unable to create temp file. Error: %s", err)
	}
	defer temp2.Close()
	defer os.Remove(temp2.Name())
	streamWriter := cipher.StreamWriter{
		S: cipher.NewCFBEncrypter(aesCipher, aesIV),
		W: temp2,
	}
	_, err = temp1.Seek(0, os.SEEK_SET)
	if err != nil {
		return nil, fmt.Errorf("Unable to seek file. Error: %s", err)
	}
	_, err = io.Copy(streamWriter, temp1)
	if err != nil {
		return nil, fmt.Errorf("Unable to encrypt file. Error: %s", err)
	}

	// Compute erasure codes
	tempStat, err := temp2.Stat()
	if err != nil {
		return nil, fmt.Errorf("Unable to stat temp file. Error: %s", err)
	}
	blockSize := (tempStat.Size() + kDefaultDataBlocks - 1) / kDefaultDataBlocks
	if blockSize > kMaxBlockSize {
		blockSize = kMaxBlockSize
	}
	// Pad file up to next nearest block size multiple if necessary.
	// Its size must be a block size multiple.
	paddingBytes := (blockSize - (tempStat.Size() % blockSize)) % blockSize
	if paddingBytes != 0 {
		err := temp2.Truncate(tempStat.Size() + paddingBytes)
		if err != nil {
			return nil, fmt.Errorf("Unable to pad file. Error: %s", err)
		}
	}
	nDataBlocks := int((tempStat.Size() + blockSize - 1) / blockSize)
	nParityBlocks := kDefaultParityBlocks
	if nDataBlocks > kDefaultDataBlocks {
		nParityBlocks = (nDataBlocks + 1) / 2
	}
	encoder, err := reedsolomon.NewStream(nDataBlocks, nParityBlocks)
	if err != nil {
		return nil, fmt.Errorf("Unable to create erasure encoder. Error: %s", err)
	}
	var blockReaders []io.Reader
	for i := 0; i < nDataBlocks; i++ {
		blockReaders = append(blockReaders, io.NewSectionReader(temp2, blockSize*int64(i), blockSize))
	}
	var parityFiles []*os.File
	for i := 0; i < nParityBlocks; i++ {
		f, err := ioutil.TempFile("", "skybin_upload")
		if err != nil {
			return nil, fmt.Errorf("Unable to create parity file. Error: %s", err)
		}
		defer f.Close()
		defer os.Remove(f.Name())
		parityFiles = append(parityFiles, f)
	}
	err = encoder.Encode(blockReaders, convertToWriterSlice(parityFiles))
	if err != nil {
		return nil, fmt.Errorf("Unable to create erasure codes. Error: %s", err)
	}

	// Prepare block metadata
	blockReaders = []io.Reader{}
	for blockNum := 0; blockNum < nDataBlocks; blockNum++ {
		blockReaders = append(blockReaders, io.NewSectionReader(temp2, blockSize*int64(blockNum), blockSize))
	}
	for i := 0; i < nParityBlocks; i++ {
		_, err := parityFiles[i].Seek(0, os.SEEK_SET)
		if err != nil {
			return nil, fmt.Errorf("Unable to seek parity file. Error: %s", err)
		}
		blockReaders = append(blockReaders, parityFiles[i])
	}

	var blocks []core.Block
	for blockNum, blockReader := range blockReaders {
		blockId, err := genId()
		if err != nil {
			return nil, fmt.Errorf("Unable to create block ID. Error: %s", err)
		}
		h := sha256.New()
		n, err := io.Copy(h, blockReader)
		if err != nil {
			return nil, fmt.Errorf("Unable to calculate block hash. Error: %s", err)
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

	// Prepare metadata before actually uploading blocks.
	uploadSize := blockSize * int64(nDataBlocks)
	for _, f := range parityFiles {
		st, err := f.Stat()
		if err != nil {
			return nil, fmt.Errorf("Unable to stat parity file. Error: %s", err)
		}
		uploadSize += st.Size()
	}
	version := &core.Version{
		Size:            finfo.Size(),
		UploadSize:      uploadSize,
		PaddingBytes:    paddingBytes,
		ModTime:         finfo.ModTime(),
		NumDataBlocks:   nDataBlocks,
		NumParityBlocks: nParityBlocks,
		Blocks:          blocks,
	}

	// Seek back to beginning of parity files
	for _, parityFile := range parityFiles {
		_, err := parityFile.Seek(0, os.SEEK_SET)
		if err != nil {
			return nil, err
		}
	}

	// Find storage. Once this is reserved, errors must be handled more carefully.
	blobs, err := r.findStorage(nDataBlocks+nParityBlocks, blockSize)
	if err != nil {
		return nil, err
	}

	// Update block locations
	for i := 0; i < len(blocks); i++ {
		blocks[i].Location = core.BlockLocation{
			ProviderId: blobs[i].ProviderId,
			Addr:       blobs[i].Addr,
			ContractId: blobs[i].ContractId,
		}
	}

	// Upload the blocks
	blockReaders = []io.Reader{}
	for blockNum := 0; blockNum < nDataBlocks; blockNum++ {
		blockReaders = append(blockReaders, io.NewSectionReader(temp2, blockSize*int64(blockNum), blockSize))
	}
	for i := 0; i < nParityBlocks; i++ {
		blockReaders = append(blockReaders, parityFiles[i])
	}
	var blockNum int
	var reader io.Reader
	for blockNum, reader = range blockReaders {
		block := blocks[blockNum]
		blob := blobs[blockNum]
		client := provider.NewClient(blob.Addr, &http.Client{})
		err = client.AuthorizeRenter(r.privKey, r.Config.RenterId)
		if err != nil {
			goto unwind
		}
		err = client.PutBlock(r.Config.RenterId, block.ID, reader)
		if err != nil {
			goto unwind
		}
	}

	return version, nil

unwind:
	for i := 0; i < blockNum; i++ {
		r.removeBlock(&blocks[i])
	}
	err2 := r.saveSnapshot()
	if err2 != nil {
		r.logger.Println("Upload: Error saving snapshot: ", err2)
	}
	return nil, err
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

	var blobs []*storageBlob
	for len(blobs) < nblocks && len(candidates) > 0 {
		idx := mathrand.Intn(len(candidates))
		candidate := candidates[idx]

		// Check if the provider is online
		client := provider.NewClient(candidate.Addr, &http.Client{})
		_, err := client.GetInfo()
		if err != nil {
			candidates = append(candidates[:idx], candidates[idx+1:]...)
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
			candidates = append(candidates[:idx], candidates[idx+1:]...)
		}
		if candidate.Amount < kMinBlobSize {
			r.freelist = append(r.freelist[:candidate.idx], r.freelist[candidate.idx+1:]...)
		}
	}

	if len(blobs) < nblocks {
		for _, blob := range blobs {
			r.addBlob(blob)
		}
		return nil, errors.New("Cannot find enough storage.")
	}

	return blobs, nil
}
