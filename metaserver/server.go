package metaserver

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/authorization"
	"skybin/core"

	"github.com/gorilla/mux"
)

// Directory where DB and error logs are dumped.
var dataDir string

// Path in dataDir where database is stored
var dbpath string

var logger *log.Logger
var router *mux.Router

// Key used to sign JWTs
// BUG(Kincaid): Signing key is still a test key.
var signingKey = []byte("secret")
var authMiddleware = authorization.GetAuthMiddleware(signingKey)

// InitServer prepares a handler for the server.
func InitServer(dataDirectory string, log *log.Logger) http.Handler {
	router := mux.NewRouter()

	dataDir = dataDirectory
	dbpath = path.Join(dataDir, "metaDB.json")
	logger = log

	// If the database exists, load it into memory.
	if _, err := os.Stat(dbpath); !os.IsNotExist(err) {
		loadDbFromFile()
	}

	authorization.InitAuth()

	router.Handle("/auth/provider", authorization.GetAuthChallengeHandler("providerID", logger)).Methods("GET")
	router.Handle("/auth/provider", authorization.GetRespondAuthChallengeHandler("providerID", logger, signingKey, getProviderPublicKey)).Methods("POST")

	router.Handle("/auth/renter", authorization.GetAuthChallengeHandler("renterID", logger)).Methods("GET")
	router.Handle("/auth/renter", authorization.GetRespondAuthChallengeHandler("renterID", logger, signingKey, getRenterPublicKey)).Methods("POST")

	router.Handle("/providers", getProvidersHandler).Methods("GET")
	router.Handle("/providers", postProviderHandler).Methods("POST")
	router.Handle("/providers/{id}", authMiddleware.Handler(getProviderHandler)).Methods("GET")

	router.Handle("/renters", postRenterHandler).Methods("POST")
	router.Handle("/renters/{id}", authMiddleware.Handler(getRenterHandler)).Methods("GET")
	router.Handle("/renters/{id}/files", authMiddleware.Handler(getRenterFilesHandler)).Methods("GET")
	router.Handle("/renters/{id}/files", authMiddleware.Handler(postRenterFileHandler)).Methods("POST")
	router.Handle("/renters/{id}/files/{fileId}", authMiddleware.Handler(getRenterFileHandler)).Methods("GET")
	router.Handle("/renters/{id}/files/{fileId}", authMiddleware.Handler(deleteRenterFileHandler)).Methods("DELETE")

	return router
}

// ServeHTTP begins serving requests from the server's router.
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Println(r.Method, r.URL)
	router.ServeHTTP(w, r)
}

type storageFile struct {
	Providers []core.Provider
	Renters   []core.Renter
}

func dumpDbToFile(providers []core.Provider, renters []core.Renter) {
	db := storageFile{Providers: providers, Renters: renters}

	dbBytes, err := json.Marshal(db)
	if err != nil {
		panic(err)
	}

	writeErr := ioutil.WriteFile(path.Join(dataDir, dbpath), dbBytes, 0644)
	if writeErr != nil {
		panic(err)
	}
}

func loadDbFromFile() {
	contents, err := ioutil.ReadFile(path.Join(dataDir, dbpath))
	if err != nil {
		panic(err)
	}

	var db storageFile
	parseErr := json.Unmarshal(contents, &db)
	if parseErr != nil {
		panic(parseErr)
	}

	providers = db.Providers
	renters = db.Renters
}
