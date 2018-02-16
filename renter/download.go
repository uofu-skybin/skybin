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
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path"
	"skybin/core"
	"skybin/provider"

	"github.com/klauspost/reedsolomon"
)

func (r *Renter) Download(fileId string, destPath string) error {
	f, err := r.Lookup(fileId)
	if err != nil {
		return err
	}
	if f.IsDir {
		return errors.New("Folder downloads not supported yet")
	}
	if len(f.Versions) == 0 {
		return errors.New("File has no versions")
	}

	// Download to home directory if no destination given
	if len(destPath) == 0 {
		destPath, err = defaultDownloadLocation(f)
		if err != nil {
			return err
		}
	}

	// Download the latest version by default
	version := &f.Versions[len(f.Versions)-1]
	return r.performDownload(f, version, destPath)
}

// Download a version of a file.
func (r *Renter) performDownload(file *core.File, version *core.Version, destPath string) error {
	var blockFiles []*os.File
	successes := 0
	failures := 0
	for i := 0; successes < version.NumDataBlocks && failures <= version.NumParityBlocks; i++ {
		temp, err := ioutil.TempFile("", "skybin_download")
		if err != nil {
			return fmt.Errorf("Cannot create temp file. Error: %s", err)
		}
		defer temp.Close()
		defer os.Remove(temp.Name())
		err = r.downloadBlock(file.OwnerID, &version.Blocks[i], temp)
		if err == nil {
			successes++
			blockFiles = append(blockFiles, temp)
		} else {
			failures++
			blockFiles = append(blockFiles, nil)
		}
	}
	if successes < version.NumDataBlocks {
		return errors.New("Failed to download enough file data blocks.")
	}
	if failures > 0 {

		// Reconstruct file from parity blocks
		for _, blockFile := range blockFiles {
			if blockFile != nil {
				_, err := blockFile.Seek(0, os.SEEK_SET)
				if err != nil {
					return fmt.Errorf("Unable to seek block file. Error: %s", err)
				}
			}
		}

		blockReaders := convertToReaderSlice(blockFiles)
		for len(blockReaders) < version.NumDataBlocks+version.NumParityBlocks {
			blockReaders = append(blockReaders, nil)
		}

		var fillFiles []*os.File
		for idx, blockReader := range blockReaders {
			var fillFile *os.File = nil
			if blockReader == nil && idx < version.NumDataBlocks {
				temp, err := ioutil.TempFile("", "skybin_download")
				if err != nil {
					return fmt.Errorf("Cannot create temp file. Error: %s", err)
				}
				defer temp.Close()
				defer os.Remove(temp.Name())
				fillFile = temp
			}
			fillFiles = append(fillFiles, fillFile)
		}
		decoder, err := reedsolomon.NewStream(version.NumDataBlocks, version.NumParityBlocks)
		if err != nil {
			return fmt.Errorf("Unable to construct decoder. Error: %s", err)
		}
		err = decoder.Reconstruct(blockReaders, convertToWriterSlice(fillFiles))
		if err != nil {
			return fmt.Errorf("Failed to reconstruct file. Error: %s", err)
		}

		for i := 0; i < version.NumDataBlocks; i++ {
			if blockFiles[i] == nil {
				blockFiles[i] = fillFiles[i]
			}
		}
		blockFiles = blockFiles[:version.NumDataBlocks]
	}

	// Download successful. Rewind the block files.
	if len(blockFiles) != version.NumDataBlocks {
		panic("block files should contain file.NumDataBlocks files")
	}
	for _, f := range blockFiles {
		_, err := f.Seek(0, os.SEEK_SET)
		if err != nil {
			return fmt.Errorf("Unable to seek block file. Error: %s", err)
		}
	}

	// Remove padding of the last block
	if version.PaddingBytes > 0 {
		f := blockFiles[len(blockFiles)-1]
		st, err := f.Stat()
		if err != nil {
			return fmt.Errorf("Unable to stat block file. Error: %s", err)
		}
		err = f.Truncate(st.Size() - version.PaddingBytes)
		if err != nil {
			return fmt.Errorf("Unable to truncate padding bytes. Error: %s", err)
		}
	}

	// Decrypt
	aesKey, aesIV, err := r.decryptEncryptionKeys(file)
	if err != nil {
		return err
	}
	aesCipher, err := aes.NewCipher(aesKey)
	if err != nil {
		return fmt.Errorf("Unable to create aes cipher. Error: %v", err)
	}
	streamReader := cipher.StreamReader{
		S: cipher.NewCFBDecrypter(aesCipher, aesIV),
		R: io.MultiReader(convertToReaderSlice(blockFiles)...),
	}
	temp2, err := ioutil.TempFile("", "skybin_download")
	if err != nil {
		return fmt.Errorf("Unable to create temp file to decrypt download. Error: %v", err)
	}
	defer temp2.Close()
	defer os.Remove(temp2.Name())
	_, err = io.Copy(temp2, streamReader)
	if err != nil {
		return fmt.Errorf("Unable to decrypt file. Error: %s", err)
	}
	_, err = temp2.Seek(0, os.SEEK_SET)
	if err != nil {
		return fmt.Errorf("Unable to seek to beginning of decrypted temp. Error: %s", err)
	}

	// Decompress
	zr, err := zlib.NewReader(temp2)
	if err != nil {
		return fmt.Errorf("Unable to initialize decompression reader. Error: %v", err)
	}
	defer zr.Close()
	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("Unable to create destination file. Error: %v", err)
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, zr)
	if err != nil {
		return fmt.Errorf("Unable to decompress file. Error: %v", err)
	}
	return nil
}

func (r *Renter) downloadBlock(renterId string, block *core.Block, out *os.File) error {
	for _, location := range block.Locations {
		client := provider.NewClient(location.Addr, &http.Client{})

		blockReader, err := client.GetBlock(renterId, block.ID)
		if err != nil {
			// TODO: Check that failure is due to a network error, not because
			// provider didn't return the block.
			continue
		}
		defer blockReader.Close()
		n, err := io.Copy(out, blockReader)
		if err != nil {
			return fmt.Errorf("Cannot write block %s to local file. Error: %s", block.ID, err)
		}
		if n != block.Size {
			return errors.New("Downloaded block has incorrect size.")
		}
		_, err = out.Seek(0, os.SEEK_SET)
		if err != nil {
			return fmt.Errorf("Error checking block hash. Error: %s", err)
		}
		h := sha256.New()
		_, err = io.Copy(h, out)
		if err != nil {
			return fmt.Errorf("Error checking block hash. Error: %s", err)
		}
		blockHash := base64.URLEncoding.EncodeToString(h.Sum(nil))
		if blockHash != block.Sha256Hash {
			return errors.New("Block hash does not match expected.")
		}
		return nil
	}
	return fmt.Errorf("Unable to download file block %s. Cannot connect to providers.", block.ID)
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

func defaultDownloadLocation(f *core.File) (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	destPath := path.Join(user.HomeDir, path.Base(f.Name))
	if _, err := os.Stat(destPath); err == nil {
		for i := 1; ; i++ {
			d := fmt.Sprintf("%s (%d)", destPath, i)
			if _, err := os.Stat(d); os.IsNotExist(err) {
				return d, nil
			}
		}
	}
	return destPath, nil
}

func convertToWriterSlice(files []*os.File) []io.Writer {
	var res []io.Writer
	for _, f := range files {
		if f == nil {
			// Must explicitly append nil since Go will otherwise
			// not treat f as nil in subsequent equality checks
			res = append(res, nil)
		} else {
			res = append(res, f)
		}

	}
	return res
}

func convertToReaderSlice(files []*os.File) []io.Reader {
	var res []io.Reader
	for _, f := range files {
		if f == nil {
			res = append(res, nil)
		} else {
			res = append(res, f)
		}
	}
	return res
}
