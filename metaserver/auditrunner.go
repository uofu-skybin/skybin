package metaserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
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
