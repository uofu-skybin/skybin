package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/core"

	"github.com/gorilla/mux"
)

// Retrieves the given renter's public RSA key.
func (server *metaServer) getRenterPublicKey(renterID string) (string, error) {
	renter, err := server.db.FindRenterByID(renterID)
	if err != nil {
		return "", err
	}
	return renter.PublicKey, nil
}

type postRenterResp struct {
	Renter core.RenterInfo `json:"provider,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (server *metaServer) postRenterHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var renter core.RenterInfo
		err := json.NewDecoder(r.Body).Decode(&renter)

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := postProviderResp{Error: "unable to parse payload"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Make sure the user supplied a public key for the provider.
		if renter.PublicKey == "" {
			w.WriteHeader(http.StatusBadRequest)
			resp := postRenterResp{Error: "must specify RSA public key"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		_, err = parsePublicKey(renter.PublicKey)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := postRenterResp{Error: "invalid RSA public key"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		renter.ID = fingerprintKey(renter.PublicKey)

		err = server.db.InsertRenter(renter)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := postRenterResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
		}
		json.NewEncoder(w).Encode(renter)
	})
}

func (server *metaServer) getRenterHandler() http.HandlerFunc {
	// BUG(kincaid): Validate that the person requesting the data is the specified renter.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		renter, err := server.db.FindRenterByID(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(renter)
	})
}
