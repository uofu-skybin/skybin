package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/core"

	"crypto/rsa"
	"skybin/util"

	"github.com/gorilla/mux"
)

// Retrieves the given renter's public RSA key.
func (server *MetaServer) getRenterPublicKey(renterID string) (*rsa.PublicKey, error) {
	renter, err := server.db.FindRenterByID(renterID)
	if err != nil {
		return nil, err
	}
	key, err := util.UnmarshalPublicKey([]byte(renter.PublicKey))
	if err != nil {
		return nil, err
	}
	return key, nil
}

type postRenterResp struct {
	Renter core.RenterInfo `json:"provider,omitempty"`
}

func (server *MetaServer) postRenterHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var renter core.RenterInfo
		err := json.NewDecoder(r.Body).Decode(&renter)

		if err != nil {
			writeErr("unable to parse payload", http.StatusBadRequest, w)
			return
		}

		// Make sure the user supplied a public key and alias for the renter.
		if renter.PublicKey == "" {
			writeErr("must specify RSA public key", http.StatusBadRequest, w)
			return
		} else if renter.Alias == "" {
			writeErr("must specify alias", http.StatusBadRequest, w)
			return
		}

		_, err = parsePublicKey(renter.PublicKey)
		if err != nil {
			writeErr("invalid RSA public key", http.StatusBadRequest, w)
			return
		}

		renter.ID = fingerprintKey(renter.PublicKey)

		err = server.db.InsertRenter(&renter)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(renter)
	})
}

type PublicRenterResp struct {
	ID        string `json:"id"`
	Alias     string `json:"alias"`
	PublicKey string `json:"publicKey"`
}

func (server *MetaServer) getRenterHandler() http.HandlerFunc {
	// BUG(kincaid): Validate that the person requesting the data is the specified renter.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		renter, err := server.db.FindRenterByID(params["id"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}

		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		if renterID, present := claims["renterID"]; present && renterID.(string) == params["id"] {
			json.NewEncoder(w).Encode(renter)
			return
		} else {
			publicInfo := PublicRenterResp{ID: renter.ID, Alias: renter.Alias, PublicKey: renter.PublicKey}
			json.NewEncoder(w).Encode(publicInfo)
		}
	})
}

func (server *MetaServer) putRenterHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		// Make sure the person making the request is the renter.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != params["id"] {
			writeErr("cannot modify other renters", http.StatusUnauthorized, w)
			return
		}

		// Make sure renter exists.
		renter, err := server.db.FindRenterByID(params["id"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		// Attempt to decode the supplied renter.
		var updatedRenter core.RenterInfo
		err = json.NewDecoder(r.Body).Decode(&updatedRenter)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}
		// Make sure the user has not changed the renter's ID or alias.
		// BUG(kincaid): Think about other fields users shouldn't change.
		// BUG(kincaid): Should the user be able to change their alias?
		if updatedRenter.ID != renter.ID {
			writeErr("must not change renter ID", http.StatusUnauthorized, w)
			return
		} else if updatedRenter.Alias != renter.Alias {
			writeErr("must not change renter alias", http.StatusUnauthorized, w)
			return
		} else if updatedRenter.PublicKey != renter.PublicKey {
			writeErr("must not change renter public key", http.StatusUnauthorized, w)
			return
		}

		// Put the new renter into the database.
		err = server.db.UpdateRenter(&updatedRenter)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		w.WriteHeader(http.StatusOK)
		resp := postRenterResp{Renter: updatedRenter}
		json.NewEncoder(w).Encode(resp)
	})
}

func (server *MetaServer) deleteRenterHandler() http.HandlerFunc {
	// BUG(kincaid): Validate that the person requesting the data is the specified renter.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		// Make sure the person making the request is the renter.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != params["id"] {
			writeErr("cannot delete other renters", http.StatusUnauthorized, w)
			return
		}

		err = server.db.DeleteRenter(params["id"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
