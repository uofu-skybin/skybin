package renter

import (
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/klauspost/reedsolomon"
	"io"
	"io/ioutil"
	mathrand "math/rand"
	"net/http"
	"os"
	"skybin/core"
	"skybin/provider"
)

func (r *Renter) Upload(srcPath string, destPath string, shouldOverwrite bool) (*core.File, error) {
	finfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	if finfo.IsDir() {
		return nil, errors.New("Folder uploads not supported yet")
	}
	if r.storageAvailable() <= finfo.Size() {
		return nil, errors.New("Not enough storage")
	}

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
		blockHash := string(h.Sum(nil))
		block := core.Block{
			ID:         blockId,
			Num:        blockNum,
			Size:       n,
			Sha256Hash: blockHash,
		}
		blocks = append(blocks, block)
	}

	// Prepare file metadata. This is done before uploading blocks to ease error handling.
	fileId, err := genId()
	if err != nil {
		return nil, err
	}
	aesKeyEncrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &r.privKey.PublicKey, aesKey, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to encrypt aes key. Error: %s", err)
	}
	aesIVEncrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &r.privKey.PublicKey, aesIV, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to encrypt aes IV. Error: %s", err)
	}
	uploadSize := blockSize * int64(nDataBlocks)
	for _, f := range parityFiles {
		st, err := f.Stat()
		if err != nil {
			return nil, fmt.Errorf("Unable to stat parity file. Error: %s", err)
		}
		uploadSize += st.Size()
	}
	file := &core.File{
		ID:              fileId,
		OwnerID:         r.Config.RenterId,
		Name:            destPath,
		IsDir:           false,
		AccessList:      make([]core.Permission, 0),
		AesKey:          string(aesKeyEncrypted),
		AesIV:           string(aesIVEncrypted),
		Versions: []core.Version{
			{
				// TODO: Include version number?
				Size: finfo.Size(),
				UploadSize: uploadSize,
				PaddingBytes: paddingBytes,
				ModTime: finfo.ModTime(),
				NumDataBlocks: nDataBlocks,
				NumParityBlocks: nParityBlocks,
				Blocks: blocks,
			},
		},
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
		blocks[i].Locations = append(blocks[i].Locations, core.BlockLocation{
			ProviderId: blobs[i].ProviderId,
			Addr:       blobs[i].Addr,
			ContractId: blobs[i].ContractId,
		})
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
		err = client.PutBlock(r.Config.RenterId, block.ID, reader)
		if err != nil {
			goto unwind
		}
	}
	err = r.saveFile(file)
	if err != nil {
		goto unwind
	}
	return file, nil

unwind:
	for i := 0; i < blockNum; i++ {
		err := r.removeBlock(&blocks[i])
		if err != nil {
			// TODO: add block to list of blocks to be removed later
		}
	}
	for _, blob := range blobs {
		r.addBlob(blob)
	}
	err2 := r.saveSnapshot()
	if err2 != nil {
		// TODO:
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

func (r *Renter) storageAvailable() int64 {
	var total int64 = 0
	for _, blob := range r.freelist {
		total += blob.Amount
	}
	return total
}
