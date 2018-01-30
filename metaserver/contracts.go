package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/core"

	"github.com/gorilla/mux"
)

type contractResp struct {
	Contract core.Contract `json:"file,omitempty"`
	Error    string        `json:"error,omitempty"`
}

func (server *metaServer) getContractsHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		// Make sure the specified renter actually exists.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := contractResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Retrieve the renter's contracts.
		contracts, err := server.db.FindContractsByRenter(renter.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println(err)
			return
		}
		json.NewEncoder(w).Encode(contracts)
	})
}

func (server *metaServer) postContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var contract core.Contract
		err := json.NewDecoder(r.Body).Decode(&contract)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := contractResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}

		params := mux.Vars(r)

		// Make sure the renter exists.
		_, err = server.db.FindRenterByID(params["renterID"])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := contractResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// BUG(kincaid): Make sure the contract's renter ID is set to that of the renter.

		// BUG(kincaid): DB will throw error if file already exists. Might want to check explicitly.
		err = server.db.InsertContract(contract)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := contractResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(contract)
	})
}

func (server *metaServer) getContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		contract, err := server.db.FindContractByID(params["contractID"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			server.logger.Println(err)
			return
		}
		json.NewEncoder(w).Encode(contract)
	})
}

func (server *metaServer) putContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var contract core.Contract
		err := json.NewDecoder(r.Body).Decode(&contract)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := contractResp{Error: "could not parse payload"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if contract.ID != params["contractID"] {
			w.WriteHeader(http.StatusUnauthorized)
			resp := contractResp{Error: "must not change contact ID"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		err = server.db.UpdateContract(contract)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := contractResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *metaServer) deleteContractHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		err := server.db.DeleteContract(params["contractID"])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
