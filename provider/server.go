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

	"fmt"
	"github.com/gorilla/mux"
)

func NewServer(provider *Provider, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := providerServer{
		provider: provider,
		logger:   logger,
		router:   router,
	}

	// API for remote renters
	router.HandleFunc("/contracts", server.postContract).Methods("POST")
	router.HandleFunc("/blocks/{blockID}", server.postBlock).Methods("POST")
	router.HandleFunc("/blocks/{blockID}", server.getBlock).Methods("GET")
	router.HandleFunc("/blocks/{blockID}", server.deleteBlock).Methods("DELETE")

	// Local API
	// TODO: Move these to the local provider server later
	router.HandleFunc("/contracts", server.getContracts).Methods("GET")
	router.HandleFunc("/info", server.getInfo).Methods("GET")
	router.HandleFunc("/activity", server.getActivity).Methods("GET")

	return &server
}

type Activity struct {
	RequestType string         `json:"requestType,omitempty"`
	BlockId     string         `json:"blockId,omitempty"`
	RenterId    string         `json:"renterId,omitempty"`
	TimeStamp   time.Time      `json:"time,omitempty"`
	Contract    *core.Contract `json:"contract,omitempty"`
}

type providerServer struct {
	provider *Provider
	logger   *log.Logger
	router   *mux.Router
	activity []Activity // Activity feed
}

const (
	// Max activity feed size
	maxActivity = 10
)

const (

	// Activity types
	negotiateType   = "NEGOTIATE CONTRACT"
	postBlockType   = "POST BLOCK"
	getBlockType    = "GET BLOCK"
	deleteBlockType = "DELETE BLOCK"
)

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
}

func (server *providerServer) postContract(w http.ResponseWriter, r *http.Request) {
	var params postContractParams
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{"Bad json"})
		return
	}

	contract := params.Contract
	if contract == nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{"No contract given"})
		return
	}

	// Sign contract
	contract.ProviderSignature = "signature"

	// Record contract and update stats
	server.provider.contracts = append(server.provider.contracts, params.Contract)
	server.provider.stats.StorageReserved += contract.StorageSpace
	err = server.provider.saveSnapshot()
	if err != nil {
		server.logger.Println("Unable to save snapshot. Error:", err)
	}

	server.writeResp(w, http.StatusCreated, &postContractResp{Contract: contract})

	activity := Activity{
		RequestType: negotiateType,
		Contract:    params.Contract,
		TimeStamp:   time.Now(),
		RenterId:    params.Contract.RenterId,
	}
	server.addActivity(activity)
}

type postBlockParams struct {
	RenterID string `json:"renterId"`
	Data     []byte `json:"data"`
}

func (server *providerServer) postBlock(w http.ResponseWriter, r *http.Request) {
	var params postBlockParams
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest,
			errorResp{Error: "Bad json"})
		return
	}

	blockID, exists := mux.Vars(r)["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			errorResp{Error: "No block given"})
		return
	}

	path := path.Join(server.provider.Homedir, "blocks", blockID)
	ioutil.WriteFile(path, params.Data, 0666)

	// Update stats
	server.provider.stats.StorageUsed += int64(len(params.Data))
	err = server.provider.saveSnapshot()
	if err != nil {
		server.logger.Println("Unable to save snapshot. Error:", err)
	}

	server.writeResp(w, http.StatusCreated, &errorResp{})

	activity := Activity{
		RequestType: postBlockType,
		BlockId:     blockID,
		RenterId:    params.RenterID,
		TimeStamp:   time.Now(),
	}
	server.addActivity(activity)
}

type getBlockResp struct {
	Data []byte `json:"data"`
}

func (server *providerServer) getBlock(w http.ResponseWriter, r *http.Request) {
	blockID, exists := mux.Vars(r)["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: "No block given"})
		return
	}

	path := path.Join(server.provider.Homedir, "blocks", blockID)
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		msg := fmt.Sprintf("Cannot find block with ID %s", blockID)
		server.writeResp(w, http.StatusBadRequest, &errorResp{Error: msg})
		return
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		msg := fmt.Sprintf("Error reading block %s: %s\n", blockID, err)
		server.logger.Println(msg)
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: msg})
		return
	}

	server.writeResp(w, http.StatusOK, &getBlockResp{Data: data})

	activity := Activity{
		RequestType: getBlockType,
		BlockId:     blockID,
		TimeStamp:   time.Now(),
		// TODO: Need this param from renter
		// RenterId:    params.RenterID,
	}
	server.addActivity(activity)
}

func (server *providerServer) deleteBlock(w http.ResponseWriter, r *http.Request) {
	blockID, exists := mux.Vars(r)["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: "No block given"})
		return
	}

	path := path.Join(server.provider.Homedir, "blocks", blockID)
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		msg := fmt.Sprintf("Cannot find block with ID %s", blockID)
		server.writeResp(w, http.StatusBadRequest, &errorResp{Error: msg})
		return
	}

	err := os.Remove(path)
	if err != nil {
		msg := fmt.Sprintf("Error deleting block %s: %s", blockID, err)
		server.logger.Println(msg)
		server.writeResp(w, http.StatusBadRequest, &errorResp{Error: msg})
		return
	}

	server.writeResp(w, http.StatusOK, &errorResp{})

	activity := Activity{
		RequestType: deleteBlockType,
		BlockId:     blockID,
		TimeStamp:   time.Now(),
		// TODO: Need this param from provider
		// RenterId:    params.RenterID,
	}
	server.addActivity(activity)

}

type getContractsResp struct {
	Contracts []*core.Contract `json:"contracts,omitempty"`
}

func (server *providerServer) getContracts(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK,
		getContractsResp{Contracts: server.provider.contracts})
}

type getInfoResp struct {
	TotalStorage    int64 `json:"providerAllocated"`
	ReservedStorage int64 `json:"providerReserved"`
	UsedStorage     int64 `json:"providerUsed"`
	FreeStorage     int64 `json:"providerFree"`
	TotalContracts  int   `json:"providerContracts"`
}

func (server *providerServer) getInfo(w http.ResponseWriter, r *http.Request) {

	reserved := server.provider.stats.StorageReserved
	used := server.provider.stats.StorageUsed
	free := reserved - used

	info := getInfoResp{
		TotalStorage:    1 << 30,
		ReservedStorage: reserved,
		UsedStorage:     used,
		FreeStorage:     free,
		TotalContracts:  len(server.provider.contracts),
	}

	server.writeResp(w, http.StatusOK, &info)
}

type getActivityResp struct {
	Activity []Activity `json:"activity,omitempty"`
}

func (server *providerServer) getActivity(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK, &getActivityResp{Activity: server.activity})
}

func (server *providerServer) addActivity(activity Activity) {
	server.activity = append(server.activity, activity)
	if len(server.activity) > maxActivity {

		// Drop the oldest activity.
		// O(N) but fine for small feed.
		server.activity = server.activity[1:]
	}
}

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
