package provider

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"skybin/core"
)

type Config struct {
	Addr         string `json:"address"`
	MetaAddr     string `json:"metaServerAddress"`
	IdentityFile string `json:"identityFile"`
}

func NewServer(config *Config, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := providerServer{
		config: config,
		logger: logger,
		router: router,
		blocks: map[string][]byte{},
	}

	router.HandleFunc("/contracts", server.postContract).Methods("POST")
	router.HandleFunc("/blocks/{blockID}", server.postBlock).Methods("POST")
	router.HandleFunc("/blocks/{blockID}", server.getBlock).Methods("GET")

	return &server
}

type providerServer struct {
	config    *Config
	logger    *log.Logger
	router    *mux.Router
	contracts []*core.Contract
	blocks    map[string][]byte
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
	server.contracts = append(server.contracts, params.Contract)
	resp := postContractResp{
		Contract: params.Contract,
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

type postBlockParams struct {
	Data []byte `json:"data"`
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
	server.blocks[blockID] = params.Data
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
	data, exists := server.blocks[blockID]
	if !exists {
		http.Error(w, "no such block", http.StatusBadRequest)
		return
	}
	resp := getBlockResp{
		Data: data,
	}
	_ = json.NewEncoder(w).Encode(&resp)
}
