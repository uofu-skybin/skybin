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

var mySigningKey = []byte("secret")

var homedir string
var dbpath string
var logger *log.Logger
var router *mux.Router

var authMiddleware = authorization.GetAuthMiddleware(mySigningKey)

func InitServer(home string, log *log.Logger) http.Handler {
	router := mux.NewRouter()

	homedir = home
	dbpath = path.Join(homedir, "metaDB.json")
	logger = log

	// If the database exists, load it into memory.
	if _, err := os.Stat(dbpath); !os.IsNotExist(err) {
		loadDbFromFile()
	}

	authorization.InitAuth()

	router.Handle("/auth/provider", authorization.GetAuthChallengeHandler("providerID", logger)).Methods("GET")
	router.Handle("/auth/provider", authorization.GetRespondAuthChallengeHandler("providerID", logger, mySigningKey, getProviderPublicKey)).Methods("POST")

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

	writeErr := ioutil.WriteFile(path.Join(homedir, dbpath), dbBytes, 0644)
	if writeErr != nil {
		panic(err)
	}
}

func loadDbFromFile() {

	contents, err := ioutil.ReadFile(path.Join(homedir, dbpath))
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
