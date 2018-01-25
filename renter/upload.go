package renter

import (
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"skybin/core"
	"skybin/provider"
	"time"
	"crypto/rsa"
)

func (r *Renter) Upload(srcPath string, destPath string, shouldOverwrite bool) (*core.File, error) {
	finfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	if finfo.IsDir() {
		return nil, errors.New("Folder uploads not supported yet")
	}

	// Find storage for the file
	blobs, err := r.findStorage(finfo.Size())
	if err != nil {
		return nil, err
	}

	var pubKey *rsa.PublicKey
	var aesKey []byte
	var aesKeyEncrypted []byte
	var aesIV []byte
	var aesIVEncrypted []byte
	var aesBlock cipher.Block
	var tempFile *os.File
	var blocks []core.Block
	var blockIdx int
	var uploadSize int64
	var fileId string
	var file *core.File

	// Generate encryption keys
	aesKey = make([]byte, 32)
	_, err = rand.Reader.Read(aesKey)
	if err != nil {
		err = fmt.Errorf("Unable to create encryption key. Error: %v", err)
		goto error
	}
	aesIV = make([]byte, aes.BlockSize)
	_, err = rand.Reader.Read(aesIV)
	if err != nil {
		err = fmt.Errorf("Unable to read initialization vector. Error: %v", err)
		goto error
	}
	fileId, err = genId()
	if err != nil {
		goto error
	}
	pubKey, err = r.loadPublicKey()
	if err != nil {
		err = fmt.Errorf("Cannot load public key. Error: %v", err)
		goto error
	}
	aesKeyEncrypted, err = rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, aesKey, nil)
	if err != nil {
		err = fmt.Errorf("Unable to encrypt aes key. Error: %v", err)
		goto error
	}
	aesIVEncrypted, err = rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, aesIV, nil)
	if err != nil {
		err = fmt.Errorf("Unable to encrypt aes IV. Error: %v", err)
		goto error
	}
	aesBlock, err = aes.NewCipher(aesKey)
	if err != nil {
		err = fmt.Errorf("Unable to create block cipher for encryption. Error: %v", err)
		goto error
	}

	// Prepare temp file for upload
	tempFile, err = prepareFile(srcPath, cipher.NewCFBEncrypter(aesBlock, aesIV))
	if err != nil {
		goto error
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Prepare blocks for upload
	blocks, err = prepareBlocks(tempFile, blobs)
	if err != nil {
		goto error
	}

	// Upload blocks
	_, err = tempFile.Seek(0, os.SEEK_SET)
	if err != nil {
		err = fmt.Errorf("Unable to seek file. Error: %v", err)
		goto error
	}
	uploadSize = 0
	for blockIdx = 0; blockIdx < len(blocks); blockIdx++ {
		block := blocks[blockIdx]
		uploadSize += block.Size
		location := block.Locations[0]
		pvdr := provider.NewClient(location.Addr, &http.Client{})
		lr := io.LimitReader(tempFile, block.Size)
		err = pvdr.PutBlock(block.ID, r.Config.RenterId, lr)
		if err != nil {
			err = fmt.Errorf("Unable to upload block. Error: %v", err)
			goto uploadError
		}
	}

	// Save file metadata
	file = &core.File{
		ID:         fileId,
		Name:       destPath,
		IsDir:      false,
		Size:       finfo.Size(),
		UploadSize: uploadSize,
		ModTime:    finfo.ModTime(),
		AccessList: []core.Permission{},
		AesKey:     string(aesKeyEncrypted),
		AesIV:      string(aesIVEncrypted),
		Blocks:     blocks,
	}
	err = r.saveFile(file)
	if err != nil {
		goto uploadError
	}

	// Finally, add back storage blobs that we didn't use
	r.reclaimBlobs(blobs, uploadSize)
	return file, nil

uploadError:
	for i := 0; i < blockIdx; i++ {
		err := removeBlock(&blocks[i])
		if err != nil {
			// TODO: append block to list to be removed later...
		}
	}

error:
	r.freelist = append(r.freelist, blobs...)
	return nil, err
}

func (r *Renter) findStorage(amount int64) ([]*storageBlob, error) {
	var n int64 = 0
	var blobs []*storageBlob
	for idx := len(r.freelist) - 1; n < amount && idx >= 0; idx-- {
		blob := r.freelist[idx]

		// Check if the provider is online and has the space we think
		client := provider.NewClient(blob.Addr, &http.Client{
			Timeout: time.Second * 3,
		})
		_, err := client.GetInfo()
		if err != nil {
			continue
		}

		// Remove blob from free list
		r.freelist = append(r.freelist[:idx], r.freelist[idx+1:]...)
		blobs = append(blobs, blob)
		n += blob.Amount
	}

	// Did we find enough space?
	if n < amount {
		r.freelist = append(r.freelist, blobs...)
		return nil, errors.New("Cannot find enough space")
	}

	return blobs, nil
}

func (r *Renter) reclaimBlobs(blobs []*storageBlob, uploadSize int64) {
	var n int64 = 0
	for _, blob := range blobs {
		if blob.Amount + n > uploadSize {
			used := uploadSize - n
			remaining := blob.Amount - used
			if remaining > kMinBlobSize {
				bcopy := *blob
				bcopy.Amount = remaining
				r.freelist = append(r.freelist, &bcopy)
			}
		}
		n += blob.Amount
	}
}

func prepareFile(srcPath string, encrypter cipher.Stream) (*os.File, error) {

	// Compress
	temp1, err := ioutil.TempFile("", "skybin_upload")
	if err != nil {
		return nil, errors.New("Unable to create temp file to prepare upload")
	}
	defer temp1.Close()
	defer os.Remove(temp1.Name())
	cw := zlib.NewWriter(temp1)
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
	_, err = temp1.Seek(0, os.SEEK_SET)
	if err != nil {
		return nil, fmt.Errorf("Unable to seek to start of temp file. Error: %s", err)
	}

	// Encrypt
	temp2, err := ioutil.TempFile("", "skybin_upload")
	if err != nil {
		return nil, fmt.Errorf("Unable to create temp file for encrypting upload. Error: %s", err)
	}

	streamWriter := cipher.StreamWriter{
		S: encrypter,
		W: temp2,
	}
	_, err = io.Copy(streamWriter, temp1)
	if err != nil {
		err = fmt.Errorf("Unable to encrypt upload. Error: %v", err)
		goto error

	}
	_, err = temp2.Seek(0, os.SEEK_SET)
	if err != nil {
		err = fmt.Errorf("Unable to seek to start of temp file. Error: %v", err)
		goto error
	}

	return temp2, nil

error:
	temp2.Close()
	os.Remove(temp2.Name())
	return nil, err
}

func prepareBlocks(tempFile *os.File, blobs []*storageBlob) ([]core.Block, error) {
	var blocks []core.Block
	for _, blob := range blobs {
		lr := io.LimitReader(tempFile, blob.Amount)
		h := sha256.New()
		blockSize, err := io.Copy(h, lr)
		if err != nil {
			return nil, fmt.Errorf("Unable to hash block. Error: %s", err)
		}
		if blockSize == 0 {

			// EOF
			return blocks, nil
		}
		blockHash := string(h.Sum(nil))
		blockId, err := genId()
		if err != nil {
			return nil, fmt.Errorf("Unable to generate block ID. Error: %s", err)
		}
		block := core.Block{
			ID:   blockId,
			Sha256Hash: blockHash,
			Size: blockSize,
			Locations: []core.BlockLocation{
				{
					ProviderId: blob.ProviderId,
					Addr:       blob.Addr,
					ContractId: blob.ContractId,
				},
			},
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func (r *Renter) saveFile(f *core.File) error {
	r.files = append(r.files, f)
	return r.saveSnapshot()
}
