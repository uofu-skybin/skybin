package provider

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/core"

	"github.com/gorilla/mux"
)

func NewServer(provider *Provider, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := providerServer{
		provider: provider,
		logger:   logger,
		router:   router,
	}

	router.HandleFunc("/contracts", server.postContract).Methods("POST")
	router.HandleFunc("/blocks/{blockID}", server.postBlock).Methods("POST")
	router.HandleFunc("/blocks/{blockID}", server.getBlock).Methods("GET")
	router.HandleFunc("/blocks/{blockID}", server.deleteBlock).Methods("DELETE")
	// TODO: Move this to the local provider server later
	router.HandleFunc("/contracts", server.getContracts).Methods("GET")

	// router.HandleFunc("/info", server.getInfo).Methods("GET")

	return &server
}

type providerServer struct {
	provider *Provider
	logger   *log.Logger
	router   *mux.Router
}

func (server *providerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.router.ServeHTTP(w, r)
}

type postContractParams struct {
	Contract *core.Contract `json:"contract"`
}

type postContractResp struct {
	Contract *core.Contract `json:"contract"`
	Error    string         `json:"error"`
}

func (server *providerServer) postContract(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("POST", r.URL)
	var params postContractParams
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	params.Contract.ProviderSignature = "my signature"
	server.provider.contracts = append(server.provider.contracts, *params.Contract)
	resp := postContractResp{
		Contract: params.Contract,
	}
	// os.MkdirAll(server.provider.Homedir, "blocks", params.Contract.RenterId)
	server.provider.saveSnapshot()
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

type getContractsResp struct {
	Contracts []core.Contract `json:"contracts,omitempty"`
	Error     string          `json:"error,omitempty"`
}

//TODO move to provider local server later
func (server *providerServer) getContracts(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("POST", r.URL)
	//TODO handle errors

	resp := getContractsResp{
		Contracts: server.provider.contracts,
	}
	_ = json.NewEncoder(w).Encode(&resp)
}

type postBlockParams struct {
	RenterID string `json:"renterID"`
	Data     []byte `json:"data"`
}

func (server *providerServer) postBlock(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("POST", r.URL)
	var params postBlockParams
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	vars := mux.Vars(r)
	blockID, exists := vars["blockID"]
	if !exists {
		http.Error(w, "no block given", http.StatusBadRequest)
		return
	}

	path := path.Join(server.provider.Homedir, "blocks", blockID)
	server.logger.Println("RenterID", params.RenterID)
	ioutil.WriteFile(path, params.Data, 0666)

	w.WriteHeader(http.StatusCreated)
}

type getBlockResp struct {
	Data []byte `json:"data"`
}

func (server *providerServer) getBlock(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("GET", r.URL)
	vars := mux.Vars(r)
	blockID, exists := vars["blockID"]
	if !exists {
		http.Error(w, "no block given", http.StatusBadRequest)
		return
	}

	path := path.Join(server.provider.Homedir, "blocks", blockID)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			//TODO: handle error on providers end
		}
		http.Error(w, "error retrieving block", http.StatusBadRequest)
		return
	}

	resp := getBlockResp{
		Data: data,
	}
	_ = json.NewEncoder(w).Encode(&resp)
}

func (server *providerServer) deleteBlock(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("DELETE", r.URL)
	vars := mux.Vars(r)
	blockID, exists := vars["blockID"]
	if !exists {
		http.Error(w, "no block given", http.StatusBadRequest)
		return
	}

	path := path.Join(server.provider.Homedir, "blocks", blockID)

	//TODO: DANGER DANGER DANGER!!! AUTHENTICATE FIRST
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			//TODO: handle error on providers end
		}
		http.Error(w, "error retrieving block", http.StatusBadRequest)
		return
	}

	// _ = json.NewEncoder(w).Encode(&resp)
}
