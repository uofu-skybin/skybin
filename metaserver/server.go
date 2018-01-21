package metaserver

import (
	"log"
	"net/http"
	"os"
	"path"
	"skybin/authorization"
	"skybin/core"

	"github.com/gorilla/mux"
)

// InitServer prepares a handler for the server.
func InitServer(dataDirectory string, logger *log.Logger) http.Handler {
	router := mux.NewRouter()

	server := &metaServer{
		dataDir:    dataDirectory,
		db:         newJsonDB(path.Join(dataDirectory, "metaDB.json")),
		router:     router,
		logger:     logger,
		authorizer: authorization.NewAuthorizer(logger),
		signingKey: []byte("secret"),
	}

	// If the database exists, load it into memory.
	if _, err := os.Stat(server.dbpath); !os.IsNotExist(err) {
		server.loadDbFromFile()
	}

	authMiddleware := authorization.GetAuthMiddleware(server.signingKey)

	router.Handle("/auth/provider", server.authorizer.GetAuthChallengeHandler("providerID")).Methods("GET")
	router.Handle("/auth/provider", server.authorizer.GetRespondAuthChallengeHandler("providerID", server.signingKey, server.getProviderPublicKey)).Methods("POST")

	router.Handle("/auth/renter", server.authorizer.GetAuthChallengeHandler("renterID")).Methods("GET")
	router.Handle("/auth/renter", server.authorizer.GetRespondAuthChallengeHandler("renterID", server.signingKey, server.getRenterPublicKey)).Methods("POST")

	router.Handle("/providers", server.getProvidersHandler()).Methods("GET")
	router.Handle("/providers", server.postProviderHandler()).Methods("POST")
	router.Handle("/providers/{id}", server.getProviderHandler()).Methods("GET")

	router.Handle("/renters", server.postRenterHandler()).Methods("POST")
	router.Handle("/renters/{id}", authMiddleware.Handler(server.getRenterHandler())).Methods("GET")
	router.Handle("/renters/{id}/files", authMiddleware.Handler(server.getRenterFilesHandler())).Methods("GET")
	router.Handle("/renters/{id}/files", authMiddleware.Handler(server.postRenterFileHandler())).Methods("POST")
	router.Handle("/renters/{id}/files/{fileId}", authMiddleware.Handler(server.getRenterFileHandler())).Methods("GET")
	router.Handle("/renters/{id}/files/{fileId}", authMiddleware.Handler(server.deleteRenterFileHandler())).Methods("DELETE")

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
