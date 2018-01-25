package metaserver

import (
	"log"
	"net/http"
	"path"
	"skybin/authorization"
	"skybin/core"

	"github.com/gorilla/mux"
)

// InitServer prepares a handler for the server.
func InitServer(dataDirectory string, logger *log.Logger) http.Handler {
	router := mux.NewRouter()

	db := newJsonDB(path.Join(dataDirectory, "metaDB.json"))

	server := &metaServer{
		dataDir:    dataDirectory,
		db:         &db,
		router:     router,
		logger:     logger,
		authorizer: authorization.NewAuthorizer(logger),
		signingKey: []byte("secret"),
	}

	authMiddleware := authorization.GetAuthMiddleware(server.signingKey)

	router.Handle("/auth/provider", server.authorizer.GetAuthChallengeHandler("providerID")).Methods("GET")
	router.Handle("/auth/provider", server.authorizer.GetRespondAuthChallengeHandler("providerID", server.signingKey, server.getProviderPublicKey)).Methods("POST")

	router.Handle("/auth/renter", server.authorizer.GetAuthChallengeHandler("renterID")).Methods("GET")
	router.Handle("/auth/renter", server.authorizer.GetRespondAuthChallengeHandler("renterID", server.signingKey, server.getRenterPublicKey)).Methods("POST")

	router.Handle("/providers", server.getProvidersHandler()).Methods("GET")
	router.Handle("/providers", server.postProviderHandler()).Methods("POST")
	router.Handle("/providers/{id}", server.getProviderHandler()).Methods("GET")
	router.Handle("/providers/{id}", authMiddleware.Handler(server.putProviderHandler())).Methods("PUT")

	router.Handle("/renters", server.postRenterHandler()).Methods("POST")
	router.Handle("/renters/{id}", authMiddleware.Handler(server.getRenterHandler())).Methods("GET")
	router.Handle("/renters/{id}", authMiddleware.Handler(server.putRenterHandler())).Methods("PUT")

	router.Handle("/renters/{id}/files", authMiddleware.Handler(server.getFilesHandler())).Methods("GET")
	router.Handle("/renters/{id}/files/{id}", authMiddleware.Handler(server.postFileHandler())).Methods("POST")
	router.Handle("/renters/{id}/files/{id}", authMiddleware.Handler(server.getFileHandler())).Methods("GET")
	router.Handle("/renters/{id}/files/{id}", authMiddleware.Handler(server.deleteFileHandler())).Methods("DELETE")
	router.Handle("/renters/{id}/files/{id}/versions", authMiddleware.Handler(server.getFileVersionsHandler())).Methods("GET")
	router.Handle("/renters/{id}/files/{id}/versions", authMiddleware.Handler(server.postFileVersionsHandler())).Methods("POST")
	router.Handle("/renters/{id}/files/{id}/versions/{version}", authMiddleware.Handler(server.getFileVersionHandler())).Methods("GET")
	router.Handle("/renters/{id}/files/{id}/versions/{version}", authMiddleware.Handler(server.putFileVersionHandler())).Methods("PUT")
	router.Handle("/renters/{id}/files/{id}/versions/{version}", authMiddleware.Handler(server.deleteFileVersionHandler())).Methods("DELETE")

	return router
}

type metaServer struct {
	dataDir    string
	db         metaDB
	providers  []core.ProviderInfo
	renters    []core.RenterInfo
	logger     *log.Logger
	router     *mux.Router
	authorizer authorization.Authorizer
	signingKey []byte
}

// ServeHTTP begins serving requests from the server's router.
func (server *metaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
	server.ServeHTTP(w, r)
}
