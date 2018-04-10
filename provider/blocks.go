package provider

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"skybin/util"
	"errors"
	"crypto/sha256"
	"encoding/base64"
)

func (server *providerServer) postBlock(w http.ResponseWriter, r *http.Request) {
	// confirm that the request has the blockid field
	blockquery, exists := r.URL.Query()["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No block given"})
		return
	}
	blockID := blockquery[0]

	renterquery, exists := r.URL.Query()["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No renter ID given"})
		return
	}
	renterID := renterquery[0]

	claims, err := util.GetTokenClaimsFromRequest(r)
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			errorResp{Error: "Failure parsing authentication token"})
		return
	}

	// Check to confirm that the authentication token matches that of the querying renter
	if claimID, present := claims["renterID"]; !present || claimID.(string) != renterID {
		server.writeResp(w, http.StatusForbidden, errorResp{Error: "Authentication token does not match renterID"})
		return
	}

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

	// Copy body to the path
	n, err := io.Copy(f, r.Body)
	if err != nil {
		server.logger.Println("Unable to write block. Error: ", err)
		server.writeResp(w, http.StatusInternalServerError,
			errorResp{Error: "Unable to save block"})
		return
	}

	// Verify that copiedfile is the correct size
	if n > spaceAvail {
		os.Remove(path)
		msg := fmt.Sprintf("Block of size %d, exceeds available storage %d", n, spaceAvail)
		server.logger.Println(msg)
		server.writeResp(w, http.StatusInsufficientStorage, errorResp{Error: msg})
		return
	}

	// Insert into Database
	err = server.provider.db.InsertBlock(renterID, blockID, n)
	if err != nil {
		os.Remove(path)
		msg := fmt.Sprintf("Failed to insert block into DB. error: %s", err)
		server.writeResp(w, http.StatusInternalServerError, errorResp{Error: msg})
		return
	}

	// Activity: non-fatal error
	err = server.provider.addActivity("upload", n)
	if err != nil {
		server.logger.Println("Failed to update activity on upload:", err)
	}

	renter.StorageUsed += n
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

	fi, err := f.Stat()
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError, &errorResp{Error: "IOError: unable to retrieve block"})
		return
	}

	err = server.provider.addActivity("download", fi.Size())
	// non-fatal error
	if err != nil {
		server.logger.Println("Failed to update activity on download:", err)
	}

	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, f)
	if err != nil {
		server.logger.Println("Unable to write block to ResponseWriter. Error: ", err)
		return
	}
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

func (server *providerServer) deleteBlock(w http.ResponseWriter, r *http.Request) {
	blockquery, exists := r.URL.Query()["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, &errorResp{Error: "No block given"})
		return
	}
	blockID := blockquery[0]

	renterquery, exists := r.URL.Query()["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No renter ID given"})
		return
	}
	renterID := renterquery[0]
	renter, exists := server.provider.renters[renterID]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			errorResp{Error: "No contracts found for renter"})
		return
	}

	claims, err := util.GetTokenClaimsFromRequest(r)
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			errorResp{Error: "Failure parsing authentication token"})
		return
	}

	// Check to confirm that the authentication token matches that of the querying renter
	if claimID, present := claims["renterID"]; !present || claimID.(string) != renterID {
		server.writeResp(w, http.StatusForbidden, errorResp{Error: "Authentication token does not match renterID"})
		return
	}

	path := path.Join(server.provider.Homedir, "blocks", renterID, blockID)
	fi, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		msg := fmt.Sprintf("Provider does not have record of block %s", blockID)
		server.writeResp(w, http.StatusInternalServerError, errorResp{Error: msg})
		return
	}

	err = os.Remove(path)
	if err != nil {
		msg := fmt.Sprintf("Provider failed to delete block %s. error: %s", blockID, err)
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: msg})
		return
	}

	err = server.provider.db.DeleteBlockById(blockID)
	if err != nil {
		msg := fmt.Sprintf("Failed to remove %s/%s from DB. error: %s", renterID, blockID, err)
		server.writeResp(w, http.StatusInternalServerError, errorResp{Error: msg})
		return
	}

	err = server.provider.addActivity("delete", fi.Size())
	// non-fatal error
	if err != nil {
		server.logger.Println("Failed to update activity on deletion:", err)
	}

	renter.StorageUsed -= fi.Size()
	server.writeResp(w, http.StatusOK, &errorResp{})
}
