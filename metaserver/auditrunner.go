package metaserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// TODO: Refactor these into their own package inside of the provider.
// Currently, we can't use them from the provider package because it causes
// an import cycle.

type postAuditParams struct {
	Nonce string `json:"nonce"`
}

type postAuditResp struct {
	Hash string `json:"hash"`
}

func auditBlock(addr, renterID, blockID, nonce string) (hash string, err error) {
	client := http.Client{}
	url := fmt.Sprintf("http://%s/blocks/audit?renterID=%s&blockID=%s", addr, renterID, blockID)
	req := postAuditParams{nonce}
	body, err := json.Marshal(&req)
	if err != nil {
		return "", err
	}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", decodeError(resp.Body)
	}
	var respMsg postAuditResp
	err = json.NewDecoder(resp.Body).Decode(&respMsg)
	if err != nil {
		return "", err
	}
	return respMsg.Hash, nil
}

func (server *MetaServer) startAuditRunner() {
	// Frequency at which the runner should be triggered (should probably put this in a config file)
	runnerFrequency := time.Minute * 5

	// Ticker triggering the runner.
	ticker := time.NewTicker(runnerFrequency)

	go func() {
		for range ticker.C {
			server.logger.Println("Running audits...")
			err := server.runAudits()
			if err != nil {
				server.logger.Println("Error when running audits:", err)
			}
		}
	}()
}

func (server *MetaServer) runAudits() error {
	// Retrieve a list of all files on the server.
	files, err := server.db.FindAllFiles()
	if err != nil {
		return err
	}

	// For each file...
	for _, file := range files {
		// server.logger.Println("Auditing blocks for file", file.ID)

		// Audit each version of the file...
		for _, version := range file.Versions {
			// server.logger.Println("Auditing version", version.Num)

			// Check that each block of the stored version is still stored properly.
			for i, block := range version.Blocks {
				// server.logger.Println("Auditing block", block.ID)

				nonceToUse := rand.Intn(len(block.Audits))
				audit := block.Audits[nonceToUse]
				res, err := auditBlock(block.Location.Addr, file.OwnerID, block.ID, audit.Nonce)
				if err != nil {
					// server.logger.Println("Error auditing block:", err)

					version.Blocks[i].AuditPassed = false
					continue
				}
				if res != audit.ExpectedHash {
					// server.logger.Println("Audit failed")
					version.Blocks[i].AuditPassed = false
				} else {
					// server.logger.Println("Audit passed")
					version.Blocks[i].AuditPassed = true
				}
			}
			err := server.db.UpdateFileVersion(file.ID, &version)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type dashboardAuditResp struct {
	Success bool `json:"success"`
}

func (server *MetaServer) getDashboardAuditHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		if len(file.Versions) < 1 {
			writeErr("file has no stored versions", http.StatusBadRequest, w)
			return
		}

		latestVersion := file.Versions[len(file.Versions)-1]
		for i, block := range latestVersion.Blocks {
			if block.ID == params["blockID"] {
				nonceToUse := rand.Intn(len(block.Audits))
				audit := block.Audits[nonceToUse]
				res, err := auditBlock(block.Location.Addr, file.OwnerID, block.ID, audit.Nonce)
				if err != nil {
					latestVersion.Blocks[i].AuditPassed = false
				}
				if res != audit.ExpectedHash {
					latestVersion.Blocks[i].AuditPassed = false
				} else {
					latestVersion.Blocks[i].AuditPassed = true
				}
				err = server.db.UpdateFileVersion(file.ID, &latestVersion)
				if err != nil {
					writeAndLogInternalError(err, w, server.logger)
					return
				}
				resp := dashboardAuditResp{latestVersion.Blocks[i].AuditPassed}
				json.NewEncoder(w).Encode(resp)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
}
