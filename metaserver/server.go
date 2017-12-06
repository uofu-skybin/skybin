package metaserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"strconv"
	"log"

	"github.com/gorilla/mux"
)

func NewServer(homedir string, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := &metaServer{
		homedir: homedir,
		dbpath: path.Join(homedir, "metaDB.json"),
		router: router,
		logger: logger,
	}

	// If the database exists, load it into memory.
	if _, err := os.Stat(server.dbpath); !os.IsNotExist(err) {
		server.loadDbFromFile()
	}

	router.HandleFunc("/providers", server.getProviders).Methods("GET")
	router.HandleFunc("/providers", server.postProvider).Methods("POST")
	router.HandleFunc("/providers/{id}", server.getProvider).Methods("GET")

	router.HandleFunc("/renters", server.postRenter).Methods("POST")
	router.HandleFunc("/renters/{id}", server.getRenter).Methods("GET")
	router.HandleFunc("/renters/{id}/files", server.getRenterFiles).Methods("GET")
	router.HandleFunc("/renters/{id}/files", server.postRenterFile).Methods("POST")
	router.HandleFunc("/renters/{id}/files/{fileId}", server.getRenterFile).Methods("GET")
	router.HandleFunc("/renters/{id}/files/{fileId}", server.deleteRenterFile).Methods("DELETE")

	return server
}

type metaServer struct {
	homedir   string
	dbpath    string
	providers []core.Provider
	renters   []core.Renter
	logger    *log.Logger
	router    *mux.Router
}

func (server *metaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
	server.router.ServeHTTP(w, r)
}

type storageFile struct {
	Providers []core.Provider
	Renters   []core.Renter
}

func (server *metaServer) dumpDbToFile(providers []core.Provider, renters []core.Renter) {
	db := storageFile{Providers: providers, Renters: renters}

	dbBytes, err := json.Marshal(db)
	if err != nil {
		panic(err)
	}

	writeErr := ioutil.WriteFile(path.Join(server.homedir, server.dbpath), dbBytes, 0644)
	if writeErr != nil {
		panic(err)
	}
}

func (server *metaServer) loadDbFromFile() {

	contents, err := ioutil.ReadFile(path.Join(server.homedir, server.dbpath))
	if err != nil {
		panic(err)
	}

	var db storageFile
	parseErr := json.Unmarshal(contents, &db)
	if parseErr != nil {
		panic(parseErr)
	}

	server.providers = db.Providers
	server.renters = db.Renters
}

type getProvidersResp struct {
	Providers []core.Provider `json:"providers"`
	Error     string          `json:"error,omitempty"`
}

func (server *metaServer) getProviders(w http.ResponseWriter, r *http.Request) {
	resp := getProvidersResp{
		Providers: server.providers,
	}
	json.NewEncoder(w).Encode(resp)
}

type postProviderResp struct {
	Provider core.Provider `json:"provider"`
	Error    string        `json:"error,omitempty"`
}

func (server *metaServer) postProvider(w http.ResponseWriter, r *http.Request) {
	var provider core.Provider
	_ = json.NewDecoder(r.Body).Decode(&provider)
	provider.ID = strconv.Itoa(len(server.providers) + 1)
	server.providers = append(server.providers, provider)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(provider)
	server.dumpDbToFile(server.providers, server.renters)
}

func (server *metaServer) getProvider(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.providers {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (server *metaServer) postRenter(w http.ResponseWriter, r *http.Request) {
	var renter core.Renter
	_ = json.NewDecoder(r.Body).Decode(&renter)
	renter.ID = strconv.Itoa(len(server.renters) + 1)
	server.renters = append(server.renters, renter)
	json.NewEncoder(w).Encode(renter)
	server.dumpDbToFile(server.providers, server.renters)
}

func (server *metaServer) getRenter(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (server *metaServer) getRenterFiles(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item.Files)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (server *metaServer) postRenterFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for i, item := range server.renters {
		if item.ID == params["id"] {
			var file core.File
			_ = json.NewDecoder(r.Body).Decode(&file)
			server.renters[i].Files = append(item.Files, file)
			json.NewEncoder(w).Encode(item.Files)
			server.dumpDbToFile(server.providers, server.renters)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (server *metaServer) getRenterFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.renters {
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
}

func (server *metaServer) deleteRenterFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.renters {
		if item.ID == params["id"] {
			for i, file := range item.Files {
				if file.ID == params["fileId"] {
					item.Files = append(item.Files[:i], item.Files[i+1:]...)
					json.NewEncoder(w).Encode(file)
					server.dumpDbToFile(server.providers, server.renters)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
}
