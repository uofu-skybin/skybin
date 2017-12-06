package provider

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"time"

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

	// TODO: Move these to the local provider server later
	router.HandleFunc("/contracts", server.getContracts).Methods("GET")
	router.HandleFunc("/info", server.getInfo).Methods("GET")
	router.HandleFunc("/activity", server.getActivity).Methods("GET")

	return &server
}

type providerServer struct {
	provider *Provider
	logger   *log.Logger
	router   *mux.Router
}
type errorResp struct {
	Error string `json:"error,omitempty"`
}

func (server *providerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
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
	// sub divide blocks into subdirs for each individual renter
	// ++ in the future this will make it really easy to determine an individual renters used space
	// -- this could potentially complicate sharing
	// os.MkdirAll(server.provider.Homedir, "blocks", params.Contract.RenterId)
	server.provider.saveSnapshot()

	activity := Activity{
		RequestType: "NEGOTIATE CONTRACT",
		Contract:    *params.Contract,
		TimeStamp:   time.Now().Format(time.RFC3339),
		RenterId:    params.Contract.RenterId,
	}
	server.provider.activity = append(server.provider.activity, activity)

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

type getContractsResp struct {
	Contracts []core.Contract `json:"contracts,omitempty"`
	Error     string          `json:"error,omitempty"`
}

//TODO move to provider local server later
func (server *providerServer) getContracts(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("GET", r.URL)
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

	activity := Activity{
		RequestType: "POST BLOCK",
		BlockId:     blockID,
		RenterId:    params.RenterID,
		TimeStamp:   time.Now().Format(time.RFC3339),
	}

	server.provider.activity = append(server.provider.activity, activity)

	// TODO: verify that renter has a contract and available storage here
	// definitely move this logic to the provider class
	ioutil.WriteFile(path, params.Data, 0666)

	w.WriteHeader(http.StatusCreated)
}

type getBlockResp struct {
	Data []byte `json:"data"`
}

func (server *providerServer) getBlock(w http.ResponseWriter, r *http.Request) {
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
			//TODO: log error on providers end
		}
		http.Error(w, "error retrieving block", http.StatusBadRequest)
		return
	}
	activity := Activity{
		RequestType: "GET BLOCK",
		BlockId:     blockID,
		TimeStamp:   time.Now().Format(time.RFC3339),
		// TODO: Need this param from renter
		// RenterId:    params.RenterID,
	}
	server.provider.activity = append(server.provider.activity, activity)

	resp := getBlockResp{
		Data: data,
	}
	_ = json.NewEncoder(w).Encode(&resp)
}

func (server *providerServer) deleteBlock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	blockID, exists := vars["blockID"]
	if !exists {
		http.Error(w, "no block given", http.StatusBadRequest)
		return
	}
	activity := Activity{
		RequestType: "DELETE BLOCK",
		BlockId:     blockID,
		TimeStamp:   time.Now().Format(time.RFC3339),
		// TODO: Need this param from provider
		// RenterId:    params.RenterID,
	}
	server.provider.activity = append(server.provider.activity, activity)

	path := path.Join(server.provider.Homedir, "blocks", blockID)

	//TODO: DANGER DANGER DANGER!!! AUTHENTICATE FIRST
	err := os.Remove(path)

	if err != nil {
		if os.IsNotExist(err) {
			//TODO: handle error on providers end
		}
		http.Error(w, "error deleting block", http.StatusBadRequest)
		return
	}
}

func (server *providerServer) getInfo(w http.ResponseWriter, r *http.Request) {

	info, err := server.provider.Info()
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: err.Error()})
		return

	}
	server.writeResp(w, http.StatusOK, info)
}

// TODO: make requests more consistent in older functions
func (server *providerServer) writeResp(w http.ResponseWriter, status int, body interface{}) {
	w.WriteHeader(status)
	data, err := json.MarshalIndent(body, "", "    ")
	if err != nil {
		server.logger.Fatalf("error: cannot to encode response. error: %s", err)
	}
	_, err = w.Write(data)
	if err != nil {
		server.logger.Fatalf("error: cannot write response body. error: %s", err)
	}

	if r, ok := body.(*errorResp); ok && len(r.Error) > 0 {
		server.logger.Print(status, r)
	} else {
		server.logger.Println(status)
	}
}

type getActivityResp struct {
	Activity []Activity `json:"activity,omitempty"`
	Error    string     `json:"error,omitempty"`
}

//TODO move to provider local server
func (server *providerServer) getActivity(w http.ResponseWriter, r *http.Request) {
	resp := getActivityResp{
		Activity: server.provider.activity,
	}
	_ = json.NewEncoder(w).Encode(&resp)
}
