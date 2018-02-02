package metaserver

import (
	"log"
	"net/http"
	"skybin/authorization"
	"skybin/core"

	"github.com/gorilla/mux"
)

// InitServer prepares a handler for the server.
func InitServer(dataDirectory string, logger *log.Logger) *MetaServer {
	router := mux.NewRouter()

	db, err := newMongoDB()
	if err != nil {
		panic(err)
	}

	server := &MetaServer{
		dataDir:    dataDirectory,
		db:         db,
		router:     router,
		logger:     logger,
		authorizer: authorization.NewAuthorizer(logger),
		signingKey: []byte("secret"),
	}

	authMiddleware := authorization.GetAuthMiddleware(server.signingKey)

	router.Handle("/auth/provider", server.authorizer.GetAuthChallengeHandler("providerID")).Methods("GET")
	router.Handle("/auth/provider", server.authorizer.GetRespondAuthChallengeHandler(
		"providerID", server.signingKey, server.getProviderPublicKey)).Methods("POST")

	router.Handle("/auth/renter", server.authorizer.GetAuthChallengeHandler("renterID")).Methods("GET")
	router.Handle("/auth/renter", server.authorizer.GetRespondAuthChallengeHandler(
		"renterID", server.signingKey, server.getRenterPublicKey)).Methods("POST")

	router.Handle("/providers", server.getProvidersHandler()).Methods("GET")
	router.Handle("/providers", server.postProviderHandler()).Methods("POST")
	router.Handle("/providers/{id}", server.getProviderHandler()).Methods("GET")
	router.Handle("/providers/{id}", authMiddleware.Handler(server.putProviderHandler())).Methods("PUT")
	router.Handle("/providers/{id}", authMiddleware.Handler(server.deleteProviderHandler())).Methods("DELETE")

	router.Handle("/renters", server.postRenterHandler()).Methods("POST")
	router.Handle("/renters/{id}", authMiddleware.Handler(server.getRenterHandler())).Methods("GET")
	router.Handle("/renters/{id}", authMiddleware.Handler(server.putRenterHandler())).Methods("PUT")
	router.Handle("/renters/{id}", authMiddleware.Handler(server.deleteRenterHandler())).Methods("DELETE")

	router.Handle("/renters/{renterID}/contracts", authMiddleware.Handler(server.getContractsHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/contracts", authMiddleware.Handler(server.postContractHandler())).Methods("POST")
	router.Handle("/renters/{renterID}/contracts/{contractID}", authMiddleware.Handler(server.getContractHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/contracts/{contractID}", authMiddleware.Handler(server.putContractHandler())).Methods("PUT")
	router.Handle("/renters/{renterID}/contracts/{contractID}", authMiddleware.Handler(server.deleteContractHandler())).Methods("DELETE")

	router.Handle("/renters/{renterID}/files", authMiddleware.Handler(server.getFilesHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files", authMiddleware.Handler(server.postFileHandler())).Methods("POST")
	router.Handle("/renters/{renterID}/files/{fileID}", authMiddleware.Handler(server.getFileHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}", authMiddleware.Handler(server.putFileHandler())).Methods("PUT")
	router.Handle("/renters/{renterID}/files/{fileID}", authMiddleware.Handler(server.deleteFileHandler())).Methods("DELETE")
	router.Handle("/renters/{renterID}/files/{fileID}/versions", authMiddleware.Handler(server.getFileVersionsHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}/versions", authMiddleware.Handler(server.postFileVersionHandler())).Methods("POST")
	router.Handle("/renters/{renterID}/files/{fileID}/versions/{version}", authMiddleware.Handler(server.getFileVersionHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}/versions/{version}", authMiddleware.Handler(server.putFileVersionHandler())).Methods("PUT")
	router.Handle("/renters/{renterID}/files/{fileID}/versions/{version}", authMiddleware.Handler(server.deleteFileVersionHandler())).Methods("DELETE")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions", authMiddleware.Handler(server.getFilePermissionsHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions", authMiddleware.Handler(server.postFilePermissionHandler())).Methods("POST")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions/{sharedID}", authMiddleware.Handler(server.getFilePermissionHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions/{sharedID}", authMiddleware.Handler(server.putFilePermissionHandler())).Methods("PUT")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions/{sharedID}", authMiddleware.Handler(server.deleteFilePermissionHandler())).Methods("DELETE")

	router.Handle("/renters/{renterID}/shared", authMiddleware.Handler(server.getSharedFilesHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/shared/{fileID}", authMiddleware.Handler(server.getFileHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/shared/{fileID}", authMiddleware.Handler(server.deleteSharedFileHandler())).Methods("DELETE")
	router.Handle("/renters/{renterID}/shared/{fileID}/versions", authMiddleware.Handler(server.getFileVersionsHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/shared/{fileID}/versions/{version}", authMiddleware.Handler(server.getFileVersionHandler())).Methods("GET")

	return server
}

type MetaServer struct {
	dataDir    string
	db         metaDB
	providers  []core.ProviderInfo
	renters    []core.RenterInfo
	logger     *log.Logger
	router     *mux.Router
	authorizer authorization.Authorizer
	signingKey []byte
}

type errorResp struct {
	Error string `json:"error"`
}

// ServeHTTP begins serving requests from the server's router.
func (server *MetaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
	server.router.ServeHTTP(w, r)
}

func (server *MetaServer) Close() {
	server.db.CloseDB()
}
