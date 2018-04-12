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

	info := core.ProviderInfo{
		ID:          server.provider.Config.ProviderID,
		PublicKey:   string(pubKeyBytes),
		Addr:        server.provider.Config.PublicApiAddr,
		SpaceAvail:  server.provider.Config.SpaceAvail - server.provider.StorageReserved,
		StorageRate: server.provider.Config.StorageRate,
	}

	server.writeResp(w, http.StatusOK, &info)
}

// TODO: stub
func (server *providerServer) postAudit(w http.ResponseWriter, r *http.Request) {
	return
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
	_, exists = server.provider.renters[renterID]
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
