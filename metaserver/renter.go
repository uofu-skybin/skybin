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
}

func (server *metaServer) postRenterHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var renter core.RenterInfo
		err := json.NewDecoder(r.Body).Decode(&renter)

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := errorResp{Error: "unable to parse payload"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Make sure the user supplied a public key and alias for the renter.
		if renter.PublicKey == "" {
			w.WriteHeader(http.StatusBadRequest)
			resp := errorResp{Error: "must specify RSA public key"}
			json.NewEncoder(w).Encode(resp)
			return
		} else if renter.Alias == "" {
			w.WriteHeader(http.StatusBadRequest)
			resp := errorResp{Error: "must specify alias"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		_, err = parsePublicKey(renter.PublicKey)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := errorResp{Error: "invalid RSA public key"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		renter.ID = fingerprintKey(renter.PublicKey)

		err = server.db.InsertRenter(&renter)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := errorResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
		}
		w.WriteHeader(http.StatusCreated)
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
			resp := errorResp{Error: "could not find renter"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		json.NewEncoder(w).Encode(renter)
	})
}

func (server *metaServer) putRenterHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		// Make sure renter exists.
		renter, err := server.db.FindRenterByID(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := errorResp{Error: "could not find renter"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Attempt to decode the supplied renter.
		var updatedRenter core.RenterInfo
		err = json.NewDecoder(r.Body).Decode(&updatedRenter)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := errorResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Make sure the user has not changed the renter's ID or alias.
		// BUG(kincaid): Think about other fields users shouldn't change.
		// BUG(kincaid): Should the user be able to change their alias?
		if updatedRenter.ID != renter.ID {
			w.WriteHeader(http.StatusUnauthorized)
			resp := errorResp{Error: "must not change renter ID"}
			json.NewEncoder(w).Encode(resp)
			return
		} else if updatedRenter.Alias != renter.Alias {
			w.WriteHeader(http.StatusUnauthorized)
			resp := errorResp{Error: "must not change renter alias"}
			json.NewEncoder(w).Encode(resp)
			return
		} else if updatedRenter.PublicKey != renter.PublicKey {
			w.WriteHeader(http.StatusUnauthorized)
			resp := errorResp{Error: "must not change renter public key"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Put the new renter into the database.
		err = server.db.UpdateRenter(&updatedRenter)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println(err)
			resp := errorResp{Error: "internal server error"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
		resp := postRenterResp{Renter: updatedRenter}
		json.NewEncoder(w).Encode(resp)
	})
}

func (server *metaServer) deleteRenterHandler() http.HandlerFunc {
	// BUG(kincaid): Validate that the person requesting the data is the specified renter.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		err := server.db.DeleteRenter(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := errorResp{Error: "could not find renter"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
