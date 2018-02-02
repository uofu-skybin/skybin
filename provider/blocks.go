package provider

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"
)

func (server *providerServer) postBlock(w http.ResponseWriter, r *http.Request) {
	blockquery, exists := r.URL.Query()["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No block given"})
		return
	}
	blockID := blockquery[0]

	// TODO: Replace this with authorization token
	renterquery, exists := r.URL.Query()["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No renter ID given"})
		return
	}
	renterID := renterquery[0]

	renter, exists := server.provider.renters[renterID]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			errorResp{Error: "Insufficient space: You have no storage reserved."})
		return
	}
	spaceAvail := renter.StorageReserved - renter.StorageUsed

	// first check block size using the http header
	if r.ContentLength > spaceAvail {
		msg := fmt.Sprintf("Block of size %d exceeds available storage %d", r.ContentLength, spaceAvail)
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: msg})
		return
	}

	// create directory for renter's blocks if necessary
	renterDir := path.Join(server.provider.Homedir, "blocks", renterID)
	if _, err := os.Stat(renterDir); os.IsNotExist(err) {
		err := os.MkdirAll(renterDir, 0700)
		if err != nil {
			server.logger.Println(err)
			server.writeResp(w, http.StatusInternalServerError,
				errorResp{Error: "Unable to save block"})
			return
		}
	}

	// create file
	path := path.Join(renterDir, blockID)
	f, err := os.Create(path)
	if err != nil {
		server.logger.Println("Unable to create block file. Error: ", err)
		server.writeResp(w, http.StatusInternalServerError,
			errorResp{Error: "Unable to save block"})
		return
	}
	defer f.Close()

	n, err := io.Copy(f, r.Body)
	if err != nil {
		server.logger.Println("Unable to write block. Error: ", err)
		server.writeResp(w, http.StatusInternalServerError,
			errorResp{Error: "Unable to save block"})
		return
	}

	// Verify that received file is the correct size
	if n > spaceAvail {
		os.Remove(path)
		msg := fmt.Sprintf("Block of size %d, exceeds available storage %d", n, spaceAvail)
		server.logger.Println(msg)
		server.writeResp(w, http.StatusInsufficientStorage, errorResp{Error: msg})
		return
	}

	// Update stats
	server.provider.stats.StorageUsed += n
	renter.StorageUsed += n
	renter.Blocks = append(renter.Blocks, &BlockInfo{BlockId: blockID, Size: n})

	activity := Activity{
		RequestType: postBlockType,
		BlockId:     blockID,
		RenterId:    renterID,
		TimeStamp:   time.Now(),
	}
	server.provider.addActivity(activity)
	err = server.provider.saveSnapshot()
	if err != nil {
		server.logger.Println("Unable to save snapshot. Error:", err)
	}
	server.writeResp(w, http.StatusCreated, &errorResp{})
}

func (server *providerServer) getBlock(w http.ResponseWriter, r *http.Request) {
	blockquery, exists := r.URL.Query()["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: "No block given"})
		return
	}
	blockID := blockquery[0]

	renterquery, exists := r.URL.Query()["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No renter ID given"})
		return
	}
	renterID := renterquery[0]

	path := path.Join(server.provider.Homedir, "blocks", renterID, blockID)
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		msg := fmt.Sprintf("Cannot find block with ID %s", blockID)
		server.writeResp(w, http.StatusBadRequest, &errorResp{Error: msg})
		return
	}

	f, err := os.Open(path)
	if err != nil {
		server.logger.Println("Unable to open block file. Error: ", err)
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: "IOError: unable to retrieve block"})
		return
	}
	defer f.Close()

	activity := Activity{
		RequestType: getBlockType,
		BlockId:     blockID,
		TimeStamp:   time.Now(),
		RenterId:    renterID,
	}
	server.provider.addActivity(activity)

	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, f)
	if err != nil {
		server.logger.Println("Unable to write block to ResponseWriter. Error: ", err)
		return
	}
}

func (server *providerServer) deleteBlock(w http.ResponseWriter, r *http.Request) {
	blockquery, exists := r.URL.Query()["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, &errorResp{Error: "No block given"})
		return
	}
	blockID := blockquery[0]

	// TODO: Replace this with authorization token
	renterquery, exists := r.URL.Query()["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No renter ID given"})
		return
	}
	renterID := renterquery[0]
	renter, exists := server.provider.renters[renterID]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			errorResp{Error: "No contracts found for given renter"})
		return
	}

	path := path.Join(server.provider.Homedir, "blocks", renterID, blockID)
	fi, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		msg := fmt.Sprintf("Cannot find block with ID %s", blockID)
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: msg})
		return
	}

	err = os.Remove(path)
	if err != nil {
		msg := fmt.Sprintf("Error deleting block with ID %s", blockID)
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: msg})
		return
	}

	for i, block := range renter.Blocks {
		if block.BlockId == blockID {
			renter.Blocks = append(renter.Blocks[:i], renter.Blocks[i+1:]...)
			server.provider.stats.StorageUsed -= fi.Size()
			renter.StorageUsed -= fi.Size()
		}
	}

	activity := Activity{
		RequestType: deleteBlockType,
		BlockId:     blockID,
		TimeStamp:   time.Now(),
		RenterId:    renterID,
	}
	server.provider.addActivity(activity)
	err = server.provider.saveSnapshot()
	if err != nil {
		server.logger.Println("Error saving snapshot: ", err)
	}
	server.writeResp(w, http.StatusOK, &errorResp{})
}
