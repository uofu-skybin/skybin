package renter

import (
	"skybin/core"
	"io"
	"skybin/provider"
	"net/http"
	"fmt"
	"os"
	"crypto/aes"
	"crypto/cipher"
	"io/ioutil"
	"compress/zlib"
	"os/user"
	"path"
	"errors"
	"crypto/sha256"
)

func (r *Renter) Download(fileId string, destPath string) error {
	f, err := r.Lookup(fileId)
	if err != nil {
		return err
	}
	if f.IsDir {
		return errors.New("Folder downloads not supported yet")
	}

	// Download to home directory if no destination given
	if len(destPath) == 0 {
		user, err := user.Current()
		if err != nil {
			return err
		}
		destPath = path.Join(user.HomeDir, f.Name)
	}

	return r.performDownload(f, destPath)
}

func (r *Renter) performDownload(f *core.File, destPath string) error {

	// Download blocks
	temp1, err := ioutil.TempFile("", "skybin_download")
	if err != nil {
		return fmt.Errorf("Unable to create temp file for download. Error: %v", err)
	}
	defer temp1.Close()
	defer os.Remove(temp1.Name())
	for _, block := range f.Blocks {
		err = downloadBlock(&block, temp1)
		if err != nil {
			return err
		}
	}
	_, err = temp1.Seek(0, os.SEEK_SET)
	if err != nil {
		return fmt.Errorf("Unable to seek to beginning of temp file. Error: %v", err)
	}

	// Check block hashes
	for _, block := range f.Blocks {
		h:= sha256.New()
		lr := io.LimitReader(temp1, block.Size)
		_, err = io.Copy(h, lr)
		if err != nil {
			return fmt.Errorf("Unable to hash block. Error: %v", err)
		}
		if string(h.Sum(nil)) != block.Sha256Hash {
			return errors.New("Block hash does not match expected.")
		}
	}
	_, err = temp1.Seek(0, os.SEEK_SET)
	if err != nil {
		return fmt.Errorf("Unable to seek to beginning of temp file. Error: %v", err)
	}

	// Decrypt
	aesKey := []byte(f.AesKey)
	aesCipher, err := aes.NewCipher(aesKey)
	if err != nil {
		return fmt.Errorf("Unable to create aes cipher. Error: %v", err)
	}
	aesIV := []byte(f.AesIV)
	streamReader := cipher.StreamReader{
		S: cipher.NewCFBDecrypter(aesCipher, aesIV),
		R: temp1,
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

		n, err := io.Copy(out, blockReader)
		if err != nil {
			return fmt.Errorf("Cannot write block %s to local file. Error: %s", block.ID, err)
		}
		if n != block.Size {
			return errors.New("Downloaded block has incorrect size.")
		}
		return nil
	}
	return fmt.Errorf("Unable to download file block %s. Cannot connect to providers.", block.ID)
}
