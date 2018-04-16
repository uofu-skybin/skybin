package provider

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
)

func (provider *Provider) StoreBlock(renterID string, blockID string, block io.Reader, blockSize int64) error {
	provider.mu.RLock()
	renter, exists := provider.renters[renterID]
	provider.mu.RUnlock()
	if !exists {
		return errors.New("Insufficient space: You have no storage reserved.")
	}
	spaceAvail := renter.StorageReserved - renter.StorageUsed

	if blockSize > spaceAvail {
		return fmt.Errorf("Block of size %d exceeds available storage %d", blockSize, spaceAvail)
	}

	// create directory for renter's blocks if necessary
	renterDir := path.Join(provider.Homedir, "blocks", renterID)
	if _, err := os.Stat(renterDir); os.IsNotExist(err) {
		err := os.MkdirAll(renterDir, 0700)
		if err != nil {
			return errors.New("Unable to save block")
		}
	}

	path := path.Join(renterDir, blockID)
	f, err := os.Create(path)
	if err != nil {
		return errors.New("Unable to save block")
	}
	defer f.Close()

	_, err = io.CopyN(f, block, blockSize)
	if err != nil {
		os.Remove(path)
		return errors.New("Unable to save block")
	}

	err = provider.db.InsertBlock(renterID, blockID, blockSize)
	if err != nil {
		os.Remove(path)
		return fmt.Errorf("Failed to insert block into DB. error: %s", err)
	}

	provider.mu.Lock()
	renter.StorageUsed += blockSize
	provider.TotalBlocks++
	provider.StorageUsed += blockSize
	provider.mu.Unlock()

	err = provider.addActivity(activityOpUpload, blockSize)
	if err != nil {
		// non-fatal
		// provider.logger.Println("Failed to update activity on upload:", err)
	}
	return nil
}

// Returns an io.ReadCloser containing the block's contents.
// If err != nil, the caller has responsibility for closing the reader.
func (provider *Provider) GetBlock(renterID, blockID string) (io.ReadCloser, error) {
	path := path.Join(provider.Homedir, "blocks", renterID, blockID)
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return nil, fmt.Errorf("Cannot find block with ID %s", blockID)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("IOError: unable to retrieve block")
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("IOError: unable to retrieve block")
	}

	err = provider.addActivity(activityOpDownload, fi.Size())
	if err != nil {
		// non-fatal
		//provider.logger.Println("Failed to update activity on download:", err)
	}

	return f, nil
}

func (p *Provider) AuditBlock(renterID, blockID, nonce string) (hash string, err error) {
	nonceBytes, err := base64.URLEncoding.DecodeString(nonce)
	if err != nil {
		return "", fmt.Errorf("Unable to decode nonce. Error: %s", err)
	}
	blockPath := path.Join(p.Homedir, "blocks", renterID, blockID)
	if _, err := os.Stat(blockPath); err != nil && os.IsNotExist(err) {
		return "", errors.New("Cannot find block")
	}
	f, err := os.Open(blockPath)
	if err != nil {
		return "", errors.New("IOError: Unable to retrieve block")
	}
	defer f.Close()
	h := sha256.New()
	h.Write(nonceBytes)
	_, err = io.Copy(h, f)
	if err != nil {
		return "", fmt.Errorf("Error reading block. Error: %s", err)
	}
	hashBytes := h.Sum(nil)
	return base64.URLEncoding.EncodeToString(hashBytes), nil
}

func (provider *Provider) DeleteBlock(renterID, blockID string) error {
	provider.mu.RLock()
	renter, exists := provider.renters[renterID]
	provider.mu.RUnlock()
	if !exists {
		return errors.New("No contracts found for given renter")
	}

	path := path.Join(provider.Homedir, "blocks", renterID, blockID)
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Block %s does not exist", blockID)
		}
		return fmt.Errorf("IOError removing block")
	}

	blockSize := fi.Size()

	err = os.Remove(path)
	if err != nil {
		return fmt.Errorf("Failed to delete block %s. error: %s", blockID, err)
	}

	err = provider.db.DeleteBlockById(blockID)
	if err != nil {
		return fmt.Errorf("Failed to remove block %s from DB. error: %s", blockID, err)
	}

	provider.mu.Lock()
	renter.StorageUsed -= blockSize
	provider.TotalBlocks--
	provider.StorageUsed -= blockSize
	provider.mu.Unlock()

	err = provider.addActivity(activityOpDelete, blockSize)
	if err != nil {
		// non-fatal
		// provider.logger.Println("Failed to update activity on deletion:", err)
	}
	return nil
}
