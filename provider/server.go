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

	authMiddleware := authorization.GetAuthMiddleware([]byte("provider"))

	// var myHandler = http.HandlerFunc(server.postContract)
	// router.Handle("/test", authMiddleware.Handler(myHandler)).Methods("GET")

	// API for remote renters
	router.HandleFunc("/contracts", server.postContract).Methods("POST")
	// router.HandleFunc("/contracts/renew", server.renewContract).Methods("POST")

	router.HandleFunc("/blocks", server.getBlock).Methods("GET")

	// TODO: add middleware
	postBlockHandler := http.HandlerFunc(server.postBlock)
	router.Handle("/blocks", authMiddleware.Handler(postBlockHandler)).Methods("POST")

	// TODO: add middleware
	deleteBlockHandler := http.HandlerFunc(server.deleteBlock)
	router.Handle("/blocks", authMiddleware.Handler(deleteBlockHandler)).Methods("DELETE")

	// router.Handle("/blocks", authMiddleware.Handler(server.deleteBlock)).Methods("DELETE")
	router.HandleFunc("/blocks/audit", server.postAudit).Methods("POST")

	router.HandleFunc("/auth", server.authorizer.GetAuthChallengeHandler("renterID")).Methods("GET")
	router.HandleFunc("/auth", server.authorizer.GetRespondAuthChallengeHandler(
		"renterID",
		util.MarshalPrivateKey(server.provider.PrivateKey),
		server.provider.getRenterPublicKey)).Methods("POST")

	// TODO: add middleware
	router.HandleFunc("/blocks", server.getBlock).Methods("GET")

	getRenterHandler := http.HandlerFunc(server.getRenter)
	router.Handle("/renter-info", authMiddleware.Handler(getRenterHandler)).Methods("GET")
	// Renters could use this to confirm the provider info from metadata
	// TODO: change provider dashboard ui to hit /stats instead of /info
	router.HandleFunc("/info", server.getInfo).Methods("GET")

	// Local API
	// TODO: Move these to the local provider server later
	// local.HandleFunc("/info", server.postInfo).Methods("POST")
	// local.HandleFunc("/stats", server.getStats).Methods("GET")
	// local.HandleFunc("/activity", server.getActivity).Methods("GET")
	// local.HandleFunc("/contracts", server.getContracts).Methods("GET")

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
		Addr:        server.provider.Config.ApiAddr,
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
	// TODO: authenticate first
	renterID, exists := mux.Vars(r)["renterID"]
	if exists {
		server.writeResp(w, http.StatusAccepted, server.provider.renters[renterID])
	}
	server.writeResp(w, http.StatusOK, getRenterResp{renter: server.provider.renters[renterID]})
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
