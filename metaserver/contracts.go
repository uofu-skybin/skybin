package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/core"
	"skybin/util"

	"github.com/gorilla/mux"
)

type contractResp struct {
	Contract core.Contract `json:"file,omitempty"`
	Error    string        `json:"error,omitempty"`
}

func (server *MetaServer) getContractsHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != params["renterID"] {
			writeErr("cannot access other users' contracts", http.StatusUnauthorized, w)
			return
		}

		// Make sure the specified renter actually exists.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		// Retrieve the renter's contracts.
		contracts, err := server.db.FindContractsByRenter(renter.ID)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		json.NewEncoder(w).Encode(contracts)
	})
}

func (server *MetaServer) postContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var contract core.Contract
		err := json.NewDecoder(r.Body).Decode(&contract)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		params := mux.Vars(r)

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		// Make sure the renter exists.
		_, err = server.db.FindRenterByID(params["renterID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		if renterID, present := claims["renterID"]; !present || renterID.(string) != contract.RenterId {
			writeErr("cannot post contracts for other users", http.StatusUnauthorized, w)
			return
		}

		// BUG(kincaid): DB will throw error if file already exists. Might want to check explicitly.
		err = server.db.InsertContract(&contract)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(contract)
	})
}

func (server *MetaServer) getContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		contract, err := server.db.FindContractByID(params["contractID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != contract.RenterId {
			writeErr("cannot retrieve other users' contracts", http.StatusUnauthorized, w)
			return
		}

		json.NewEncoder(w).Encode(contract)
	})
}

func (server *MetaServer) putContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var contract core.Contract
		err := json.NewDecoder(r.Body).Decode(&contract)
		if err != nil {
			writeErr("could not parse payload", http.StatusBadRequest, w)
			return
		}

		if contract.ID != params["contractID"] {
			writeErr("must not change contact ID", http.StatusUnauthorized, w)
			return
		}

		oldContract, err := server.db.FindContractByID(contract.ID)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the contract.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != oldContract.RenterId {
			writeErr("cannot modify other users' contracts", http.StatusUnauthorized, w)
			return
		}

		err = server.db.UpdateContract(&contract)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *MetaServer) deleteContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		contract, err := server.db.FindContractByID(params["contractID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the contract.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != contract.RenterId {
			writeErr("cannot delete other users' contracts", http.StatusUnauthorized, w)
			return
		}

		err = server.db.DeleteContract(params["contractID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
