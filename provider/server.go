package provider

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"skybin/authorization"
	"skybin/core"
	"skybin/util"

	"github.com/gorilla/mux"
	"io"
	"strconv"
)

type providerServer struct {
	provider   *Provider
	logger     *log.Logger
	router     *mux.Router
	authorizer authorization.Authorizer
}

// Create a new public API server for given Provider
func NewServer(provider *Provider, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := providerServer{
		provider:   provider,
		logger:     logger,
		router:     router,
		authorizer: authorization.NewAuthorizer(logger),
	}

	authMiddleware := authorization.GetAuthMiddleware(util.MarshalPrivateKey(server.provider.privKey))
	router.Handle("/auth/renter", server.authorizer.GetAuthChallengeHandler("renterID")).Methods("GET")
	router.Handle("/auth/renter", server.authorizer.GetRespondAuthChallengeHandler(
		"renterID",
		util.MarshalPrivateKey(server.provider.privKey),
		server.provider.getRenterPublicKey)).Methods("POST")

	router.HandleFunc("/contracts", server.postContract).Methods("POST")
	router.HandleFunc("/blocks", server.getBlock).Methods("GET")
	router.Handle("/blocks", authMiddleware.Handler(http.HandlerFunc(server.postBlock))).Methods("POST")
	router.Handle("/blocks", authMiddleware.Handler(http.HandlerFunc(server.deleteBlock))).Methods("DELETE")
	router.HandleFunc("/blocks/audit", server.postAudit).Methods("POST")
	router.Handle("/renter-info", authMiddleware.Handler(http.HandlerFunc(server.getRenter))).Methods("GET")
	router.HandleFunc("/info", server.getInfo).Methods("GET")

	return &server
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

	proposal := params.Contract
	if proposal == nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{"No contract given"})
		return
	}

	signedContract, err := server.provider.NegotiateContract(proposal)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{err.Error()})
		return
	}
	server.writeResp(w, http.StatusCreated, &postContractResp{Contract: signedContract})
}

func (server *providerServer) postBlock(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	renterquery, exists := query["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{"No renter ID given"})
		return
	}
	renterID := renterquery[0]

	blockquery, exists := query["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{"No block given"})
		return
	}
	blockID := blockquery[0]

	sizequery, exists := query["size"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{"No block size given"})
		return
	}
	size, err := strconv.ParseInt(sizequery[0], 10, 64)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest, errorResp{"Block size must be an integer"})
		return
	}

	claims, err := util.GetTokenClaimsFromRequest(r)
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			errorResp{Error: "Failure parsing authentication token"})
		return
	}

	// Check to confirm that the authentication token matches that of the querying renter
	if claimID, present := claims["renterID"]; !present || claimID.(string) != renterID {
		server.writeResp(w, http.StatusForbidden,
			errorResp{Error: "Authentication token does not match renterID"})
		return
	}

	err = server.provider.StoreBlock(renterID, blockID, r.Body, size)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest, &errorResp{err.Error()})
		return
	}

	server.writeResp(w, http.StatusCreated, &errorResp{})
}

func (server *providerServer) getBlock(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	renterquery, exists := query["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No renter ID given"})
		return
	}
	renterID := renterquery[0]

	blockquery, exists := query["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "No block given"})
		return
	}
	blockID := blockquery[0]

	block, err := server.provider.GetBlock(renterID, blockID)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest, errorResp{err.Error()})
	}
	defer block.Close()

	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, block)
	if err != nil {
		server.logger.Println("Unable to read block. Error: ", err)
	}
}

func (server *providerServer) deleteBlock(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	renterquery, exists := query["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{"No renter ID given"})
		return
	}
	renterID := renterquery[0]

	blockquery, exists := query["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, &errorResp{"No block given"})
		return
	}
	blockID := blockquery[0]

	claims, err := util.GetTokenClaimsFromRequest(r)
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			errorResp{Error: "Failure parsing authentication token"})
		return
	}

	if claimID, present := claims["renterID"]; !present || claimID.(string) != renterID {
		server.writeResp(w, http.StatusForbidden, errorResp{"Authentication token does not match renterID"})
		return
	}

	err = server.provider.DeleteBlock(renterID, blockID)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest, errorResp{err.Error()})
	}

	server.writeResp(w, http.StatusOK, &errorResp{})
}

// This info object is different than the info object for local provider which serves
// as a means to populate the provider dashboard
func (server *providerServer) getInfo(w http.ResponseWriter, r *http.Request) {
	pubKeyBytes, err := util.MarshalPublicKey(&server.provider.privKey.PublicKey)
	if err != nil {
		server.logger.Println("Unable to marshal public key. Error: ", err)
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{"Unable to marshal public key"})
		return
	}

	// TODO: Call the appropriate method to retrieve this.
	server.provider.mu.RLock()
	info := core.ProviderInfo{
		ID:          server.provider.Config.ProviderID,
		PublicKey:   string(pubKeyBytes),
		Addr:        server.provider.Config.PublicApiAddr,
		SpaceAvail:  server.provider.Config.SpaceAvail - server.provider.StorageReserved,
		StorageRate: server.provider.Config.StorageRate,
	}
	server.provider.mu.RUnlock()

	server.writeResp(w, http.StatusOK, &info)
}

type postAuditParams struct {
	Nonce string `json:"nonce"`
}

type postAuditResp struct {
	Hash string `json:"hash"`
}

func (server *providerServer) postAudit(w http.ResponseWriter, r *http.Request) {
	renterQuery, exists := r.URL.Query()["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			errorResp{"No renter ID given"})
		return
	}
	renterID := renterQuery[0]
	blockQuery, exists := r.URL.Query()["blockID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			errorResp{"No block ID given"})
		return
	}
	blockID := blockQuery[0]
	var params postAuditParams
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{"Bad request json"})
		return
	}
	hash, err := server.provider.AuditBlock(renterID, blockID, params.Nonce)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{err.Error()})
		return
	}
	server.writeResp(w, http.StatusOK, &postAuditResp{hash})
}

type getRenterResp struct {
	StorageReserved int64            `json:"storageReserved"`
	StorageUsed     int64            `json:"storageUsed"`
	Contracts       []*core.Contract `json:"contracts"`
	Blocks          []*blockInfo     `json:"blocks"`
}

func (server *providerServer) getRenter(w http.ResponseWriter, r *http.Request) {
	renterID, exists := mux.Vars(r)["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			errorResp{Error: "Requested Renter ID does not exist on provider"})
		return
	}

	claims, err := util.GetTokenClaimsFromRequest(r)
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			errorResp{Error: "Failure parsing authentication token"})
		return
	}

	// Check to confirm that the authentication token matches that of the querying renter
	if claimID, present := claims["renterID"]; !present || claimID.(string) != renterID {
		server.writeResp(w, http.StatusForbidden,
			errorResp{Error: "Authentication token does not match renterID"})
		return
	}

	// TODO: this might not be necessary at this point
	server.provider.mu.RLock()
	_, exists = server.provider.renters[renterID]
	server.provider.mu.RUnlock()
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			errorResp{Error: "Provider has no record for this renter"})
		return
	}
	contracts, err := server.provider.db.GetContractsByRenter(renterID)
	if err != nil {
		msg := fmt.Sprintf("Failed to get contracts from DB for renter %s. Error: %s", renterID, err)
		server.writeResp(w, http.StatusInternalServerError, errorResp{Error: msg})
	}
	blocks, err := server.provider.db.GetBlocksByRenter(renterID)
	if err != nil {
		msg := fmt.Sprintf("Failed to get blocks from DB for renter %s. Error: %s", renterID, err)
		server.writeResp(w, http.StatusInternalServerError, errorResp{Error: msg})
	}
	resp := &getRenterResp{
		StorageReserved: server.provider.renters[renterID].StorageReserved,
		StorageUsed:     server.provider.renters[renterID].StorageUsed,
		Contracts:       contracts,
		Blocks:          blocks,
	}

	server.writeResp(w, http.StatusOK, resp)
}

func (server *providerServer) writeResp(w http.ResponseWriter, status int, body interface{}) {
	w.WriteHeader(status)
	data, err := json.MarshalIndent(body, "", "    ")
	if err != nil {
		server.logger.Fatalf("error: cannot encode response. error: %s", err)
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
