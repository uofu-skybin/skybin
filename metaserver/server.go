package metaserver

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/authorization"
	"skybin/core"
	"strconv"

	"github.com/gorilla/mux"
)

var mySigningKey = []byte("secret")

var homedir string
var dbpath string
var providers []core.Provider
var renters []core.Renter
var logger *log.Logger
var router *mux.Router
var handshakes map[string]handshake

func InitServer(home string, log *log.Logger) http.Handler {
	router := mux.NewRouter()

	homedir = home
	dbpath = path.Join(homedir, "metaDB.json")
	logger = log
	handshakes = make(map[string]handshake)

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

type handshake struct {
	nonce      string
	providerID string
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

func getProviderPublicKey(providerID string) (string, error) {
	for _, item := range providers {
		if item.ID == providerID {
			return item.PublicKey, nil
		}
	}
	return "", errors.New("Could not locate provider with given ID.")
}

var authMiddleware = authorization.GetAuthMiddleware(mySigningKey)

type getProvidersResp struct {
	Providers []core.Provider `json:"providers"`
	Error     string          `json:"error,omitempty"`
}

var getProvidersHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := getProvidersResp{
		Providers: providers,
	}
	json.NewEncoder(w).Encode(resp)
})

type postProviderResp struct {
	Provider core.Provider `json:"provider"`
	Error    string        `json:"error,omitempty"`
}

var postProviderHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	var provider core.Provider
	_ = json.NewDecoder(r.Body).Decode(&provider)
	provider.ID = strconv.Itoa(len(providers) + 1)
	providers = append(providers, provider)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(provider)
	dumpDbToFile(providers, renters)
})

var getProviderHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range providers {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
})

var postRenterHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	var renter core.Renter
	_ = json.NewDecoder(r.Body).Decode(&renter)
	renter.ID = strconv.Itoa(len(renters) + 1)
	renters = append(renters, renter)
	json.NewEncoder(w).Encode(renter)
	dumpDbToFile(providers, renters)
})

var getRenterHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
})

var getRenterFilesHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item.Files)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
})

var postRenterFileHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for i, item := range renters {
		if item.ID == params["id"] {
			var file core.File
			_ = json.NewDecoder(r.Body).Decode(&file)
			renters[i].Files = append(item.Files, file)
			json.NewEncoder(w).Encode(item.Files)
			dumpDbToFile(providers, renters)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
})

var getRenterFileHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			for _, file := range item.Files {
				if file.ID == params["fileId"] {
					json.NewEncoder(w).Encode(file)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
})

var deleteRenterFileHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			for i, file := range item.Files {
				if file.ID == params["fileId"] {
					item.Files = append(item.Files[:i], item.Files[i+1:]...)
					json.NewEncoder(w).Encode(file)
					dumpDbToFile(providers, renters)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
})
