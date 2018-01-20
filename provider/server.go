package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"time"

	"github.com/gorilla/mux"
)

type providerServer struct {
	provider *Provider
	logger   *log.Logger
	router   *mux.Router
	activity []Activity // Activity feed
}

func NewServer(provider *Provider, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := providerServer{
		provider: provider,
		logger:   logger,
		router:   router,
	}

	// API for remote renters
	router.HandleFunc("/contracts", server.postContract).Methods("POST")
	router.HandleFunc("/blocks/{renterID}/{blockID}", server.postBlock).Methods("POST")
	router.HandleFunc("/blocks/{renterID}/{blockID}", server.getBlock).Methods("GET")
	router.HandleFunc("/blocks/{renterID}/{blockID}", server.deleteBlock).Methods("DELETE")

	// router.HandleFunc("/auth", server.getContracts).Methods("GET")
	// router.HandleFunc("/auth", server.getContracts).Methods("POST")
	router.HandleFunc("/renter-info/{renterID}", server.getRenter).Methods("GET")

	// Local API
	// TODO: Move these to the local provider server later
	// router.HandleFunc("/info", server.getInfo).Methods("POST")
	router.HandleFunc("/info", server.getInfo).Methods("GET")
	router.HandleFunc("/activity", server.getActivity).Methods("GET")
	router.HandleFunc("/contracts", server.getContracts).Methods("GET")

	return &server
}
func (server *providerServer) getRenter(w http.ResponseWriter, r *http.Request) {
	renterID, exists := mux.Vars(r)["renterID"]
	if exists {
		server.writeResp(w, http.StatusAccepted, server.provider.renters[renterID])
	}
}

func (server *providerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
	server.router.ServeHTTP(w, r)
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

	resp, err := server.provider.negotiateContract(contract)
	server.writeResp(w, http.StatusCreated, &postContractResp{Contract: resp})
}

func (server *providerServer) postBlock(w http.ResponseWriter, r *http.Request) {
	blockID, exists := mux.Vars(r)["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No block given"})
		return
	}

	// TODO: Replace this with authorization token
	renterID, exists := mux.Vars(r)["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No renter ID given"})
		return
	}

	renter := server.provider.renters[renterID]
	avail := renter.StorageReserved - renter.StorageUsed
	fmt.Println(renter)
	// check block size using the header
	if r.ContentLength > avail {
		msg := fmt.Sprintf("Block of size %d, exceeds available storage %d", r.ContentLength, avail)
		server.writeResp(w, http.StatusInsufficientStorage, errorResp{Error: msg})
		return
	}

	// create file
	path := path.Join(server.provider.Homedir, "blocks", renterID, blockID)
	f, err := os.Create(path)
	if err != nil {
		server.logger.Println("Unable to create block file. Error: ", err)
		server.writeResp(w, http.StatusInternalServerError, errorResp{Error: "Unable to save block"})
		return
	}
	defer f.Close()

	n, err := io.Copy(f, r.Body)
	if err != nil {
		server.logger.Println("Unable to write block. Error: ", err)
		server.writeResp(w, http.StatusInternalServerError, errorResp{Error: "Unable to save block"})
		return
	}

	// check saved file size vs the available storage
	if int64(n) > avail {
		os.Remove(path)
		msg := fmt.Sprintf("Block of size %d, exceeds available storage %d", int64(n), avail)
		server.writeResp(w, http.StatusInsufficientStorage, errorResp{Error: msg})
		return
	}

	// Update stats
	server.provider.stats.StorageUsed += int64(n)
	renter.StorageUsed += int64(n)
	server.provider.renters[renterID] = renter

	err = server.provider.saveSnapshot()
	if err != nil {
		server.logger.Println("Unable to save snapshot. Error:", err)
	}

	activity := Activity{
		RequestType: postBlockType,
		BlockId:     blockID,
		RenterId:    renterID,
		TimeStamp:   time.Now(),
	}
	server.provider.addActivity(activity)

	server.writeResp(w, http.StatusCreated, &errorResp{})
}

func (server *providerServer) getBlock(w http.ResponseWriter, r *http.Request) {
	blockID, exists := mux.Vars(r)["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: "No block given"})
		return
	}
	renterID, exists := mux.Vars(r)["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No renter ID given"})
		return
	}

	path := path.Join(server.provider.Homedir, "blocks", renterID, blockID)
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		msg := fmt.Sprintf("Cannot find block with ID %s", blockID)
		server.writeResp(w, http.StatusBadRequest, &errorResp{Error: msg})
		return
	}

	f, err := os.Open(path)
	if err != nil {
		server.logger.Println("Unable to open block file. Error: ", err)
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: "IOError: unable to retrieve block"})
		return
	}
	defer f.Close()

	activity := Activity{
		RequestType: getBlockType,
		BlockId:     blockID,
		TimeStamp:   time.Now(),
		// TODO: Need this param from renter
		// RenterId:    params.RenterID,
	}
	server.provider.addActivity(activity)

	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, f)
	if err != nil {
		server.logger.Println("Unable to write block to ResponseWriter. Error: ", err)
		return
	}
}

func (server *providerServer) deleteBlock(w http.ResponseWriter, r *http.Request) {
	blockID, exists := mux.Vars(r)["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: "No block given"})
		return
	}
	renterID, exists := mux.Vars(r)["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No renterID given"})
		return
	}

	// err := server.provider.removeBlock(renterID, blockID)
	// if err != nil {
	// 	// TODO: Tune errors and error statusses?
	// 	server.writeResp(w, http.StatusForbidden, &errorResp{})
	// 	return
	// } // TODO: we need to get the renter id and authenticate here first

	path := path.Join(server.provider.Homedir, "blocks", renterID, blockID)
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		msg := fmt.Sprintf("Cannot find block with ID %s", blockID)
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: msg})
		return
	}

	err := os.Remove(path)
	if err != nil {
		msg := fmt.Sprintf("Error deleting block with ID %s", blockID)
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: msg})
		return
	}

	activity := Activity{
		RequestType: deleteBlockType,
		BlockId:     blockID,
		TimeStamp:   time.Now(),
		// RenterId:    params.RenterID,
	}
	server.provider.addActivity(activity)

	server.writeResp(w, http.StatusOK, &errorResp{"Block Deleted"})

}

func (server *providerServer) getContracts(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK,
		getContractsResp{Contracts: server.provider.contracts})
}

func (server *providerServer) getInfo(w http.ResponseWriter, r *http.Request) {

	reserved := server.provider.stats.StorageReserved
	used := server.provider.stats.StorageUsed
	free := reserved - used

	info := getInfoResp{
		ProviderId:      server.provider.Config.ProviderID,
		TotalStorage:    1 << 30,
		ReservedStorage: reserved,
		UsedStorage:     used,
		FreeStorage:     free,
		TotalContracts:  len(server.provider.contracts),
	}

	server.writeResp(w, http.StatusOK, &info)
}

func (server *providerServer) getActivity(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK, &getActivityResp{Activity: server.provider.activity})
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

type errorResp struct {
	Error string `json:"error,omitempty"`
}
type postContractParams struct {
	Contract *core.Contract `json:"contract"`
}
type postContractResp struct {
	Contract *core.Contract `json:"contract"`
}
type getInfoResp struct {
	ProviderId      string `json:"providerId"`
	TotalStorage    int64  `json:"providerAllocated"`
	ReservedStorage int64  `json:"providerReserved"`
	UsedStorage     int64  `json:"providerUsed"`
	FreeStorage     int64  `json:"providerFree"`
	TotalContracts  int    `json:"providerContracts"`
}
type getContractsResp struct {
	Contracts []*core.Contract `json:"contracts"`
}
type getRenterResp struct {
	renter renterStats `json:"renterstats"`
}
type getActivityResp struct {
	Activity []Activity `json:"activity"`
}
