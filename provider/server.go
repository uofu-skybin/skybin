package provider

import (
	"encoding/json"
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

func NewServer(provider *Provider, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := providerServer{
		provider:   provider,
		logger:     logger,
		router:     router,
		authorizer: authorization.NewAuthorizer(logger),
	}

	authMiddleware := authorization.GetAuthMiddleware(util.MarshalPrivateKey(server.provider.PrivateKey))

	// API for remote renters
	router.HandleFunc("/contracts", server.postContract).Methods("POST")
	router.HandleFunc("/blocks", server.getBlock).Methods("GET")
	router.Handle("/blocks", authMiddleware.Handler(server.postBlockHandler())).Methods("POST")
	router.Handle("/blocks", authMiddleware.Handler(server.deleteBlockHandler())).Methods("DELETE")

	router.HandleFunc("/blocks/audit", server.postAudit).Methods("POST")

	router.Handle("/auth/renter", server.authorizer.GetAuthChallengeHandler("renterID")).Methods("GET")
	router.Handle("/auth/renter", server.authorizer.GetRespondAuthChallengeHandler(
		"renterID",
		util.MarshalPrivateKey(server.provider.PrivateKey),
		server.provider.getRenterPublicKey)).Methods("POST")

	router.HandleFunc("/blocks", server.getBlock).Methods("GET")

	getRenterHandler := http.HandlerFunc(server.getRenter)
	router.Handle("/renter-info", authMiddleware.Handler(getRenterHandler)).Methods("GET")
	// Renters could use this to confirm the provider info from metadata
	router.HandleFunc("/info", server.getInfo).Methods("GET")

	return &server
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

	proposal := params.Contract
	if proposal == nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{"No contract given"})
		return
	}

	signedContract, err := server.provider.negotiateContract(proposal)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{err.Error()})
		return
	}
	server.writeResp(w, http.StatusCreated, &postContractResp{Contract: signedContract})

}

// Converted getStats to return the information for the provider dash
// This will instead just return the core.ProviderInfo object
func (server *providerServer) getInfo(w http.ResponseWriter, r *http.Request) {

	info := core.ProviderInfo{
		ID:          server.provider.Config.ProviderID,
		PublicKey:   "string",
		Addr:        server.provider.Config.PublicApiAddr,
		SpaceAvail:  9999999999 - server.provider.stats.StorageReserved,
		StorageRate: 1,
	}

	server.writeResp(w, http.StatusOK, &info)
}

// TODO: Fill out stub
func (server *providerServer) postAudit(w http.ResponseWriter, r *http.Request) {
	return
}

func (server *providerServer) getRenter(w http.ResponseWriter, r *http.Request) {
	renterID, exists := mux.Vars(r)["renterID"]
	if !exists {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "Requested Renter ID does not exist on provider"})
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
		server.writeResp(w, http.StatusForbidden, errorResp{Error: "Authentication token does not match renterID"})
		return
	}

	renter, exists := server.provider.renters[renterID]
	if !exists {
		server.writeResp(w, http.StatusBadRequest,
			errorResp{Error: "Provider has no record for this renter"})
		return
	}

	server.writeResp(w, http.StatusOK, getRenterResp{renter: renter})
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

// Responses
type errorResp struct {
	Error string `json:"error,omitempty"`
}
type postContractParams struct {
	Contract *core.Contract `json:"contract"`
}
type postContractResp struct {
	Contract *core.Contract `json:"contract"`
}
type getContractsResp struct {
	Contracts []*core.Contract `json:"contracts"`
}
type getRenterResp struct {
	renter *RenterInfo `json:"renter-info"`
}
