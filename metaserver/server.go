package metaserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"skybin/core"
	"strconv"

	"github.com/gorilla/mux"
)

const filePath = "db.json"

var providers []core.Provider
var renters []core.Renter

func NewServer() http.Handler {

	// If the database exists, load it into memory.
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		loadDbFromFile()
	}

	router := mux.NewRouter()

	router.HandleFunc("/providers", getProviders).Methods("GET")
	router.HandleFunc("/providers", postProvider).Methods("POST")
	router.HandleFunc("/providers/{id}", getProvider).Methods("GET")

	router.HandleFunc("/renters", postRenter).Methods("POST")
	router.HandleFunc("/renters/{id}", getRenter).Methods("GET")
	router.HandleFunc("/renters/{id}/files", getRenterFiles).Methods("GET")
	router.HandleFunc("/renters/{id}/files", postRenterFile).Methods("POST")
	router.HandleFunc("/renters/{id}/files/{fileId}", getRenterFile).Methods("GET")
	router.HandleFunc("/renters/{id}/files/{fileId}", delteRenterFile).Methods("DELETE")

	return router
}

type storageFile struct {
	Providers []core.Provider
	Renters   []core.Renter
}

func dumpDbToFile(providers []core.Provider, renters []core.Renter) {
	println("Dumping database to", filePath, "...")
	db := storageFile{Providers: providers, Renters: renters}

	dbBytes, err := json.Marshal(db)
	if err != nil {
		panic(err)
	}

	writeErr := ioutil.WriteFile(filePath, dbBytes, 0644)
	if writeErr != nil {
		panic(err)
	}
}

func loadDbFromFile() {
	println("Loading renter/provider database from", filePath, "...")

	contents, err := ioutil.ReadFile(filePath)
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

type getProvidersResp struct {
	Providers []core.Provider `json:"providers"`
	Error     string          `json:"error,omitempty"`
}

func getProviders(w http.ResponseWriter, r *http.Request) {
	resp := getProvidersResp{
		Providers: providers,
	}
	json.NewEncoder(w).Encode(resp)
}

type postProviderResp struct {
	Provider core.Provider `json:"provider"`
	Error    string        `json:"error,omitempty"`
}

func postProvider(w http.ResponseWriter, r *http.Request) {
	var provider core.Provider
	_ = json.NewDecoder(r.Body).Decode(&provider)
	provider.ID = strconv.Itoa(len(providers) + 1)
	providers = append(providers, provider)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(provider)
	dumpDbToFile(providers, renters)
}

func getProvider(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range providers {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func postRenter(w http.ResponseWriter, r *http.Request) {
	var renter core.Renter
	_ = json.NewDecoder(r.Body).Decode(&renter)
	renter.ID = strconv.Itoa(len(renters) + 1)
	renters = append(renters, renter)
	json.NewEncoder(w).Encode(renter)
	dumpDbToFile(providers, renters)
}

func getRenter(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func getRenterFiles(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item.Files)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func postRenterFile(w http.ResponseWriter, r *http.Request) {
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
}

func getRenterFile(w http.ResponseWriter, r *http.Request) {
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
}

func delteRenterFile(w http.ResponseWriter, r *http.Request) {
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
}
