package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"skybin/core"

	"github.com/gorilla/mux"
	"flag"
)

const filePath = "db.json"

var providers []core.Provider
var renters []core.Renter

type StorageFile struct {
	Providers []core.Provider
	Renters   []core.Renter
}

func dumpDbToFile(providers []core.Provider, renters []core.Renter) {
	println("Dumping database to", filePath, "...")
	db := StorageFile{Providers: providers, Renters: renters}

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

	var db StorageFile
	parseErr := json.Unmarshal(contents, &db)
	if parseErr != nil {
		panic(parseErr)
	}

	providers = db.Providers
	renters = db.Renters
}

// our main function
func main() {
	addrFlag := flag.String("addr", "", "address to run on (host:port)")
	flag.Parse()

	addr := core.DefaultMetaAddr
	if len(*addrFlag) > 0 {
		addr = *addrFlag
	}

	// If the database exists, load it into memory.
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		loadDbFromFile()
	}

	router := mux.NewRouter()

	providers = append(providers, core.Provider{ID: "1", PublicKey: "test", Host: "test", Port: 2, SpaceAvail: 50, StorageRate: 5})

	router.HandleFunc("/providers", GetProviders).Methods("GET")
	router.HandleFunc("/providers", PostProvider).Methods("POST")p
	router.HandleFunc("/providers/{id}", GetProvider).Methods("GET")

	router.HandleFunc("/renters", PostRenter).Methods("POST")
	router.HandleFunc("/renters/{id}", GetRenter).Methods("GET")
	router.HandleFunc("/renters/{id}/files", GetRenterFiles).Methods("GET")
	router.HandleFunc("/renters/{id}/files", PostRenterFile).Methods("POST")
	router.HandleFunc("/renters/{id}/files/{fileId}", GetRenterFile).Methods("GET")
	router.HandleFunc("/renters/{id}/files/{fileId}", DeleteRenterFile).Methods("DELETE")

	log.Fatal(http.ListenAndServe(addr, router))
}

func GetProviders(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(providers)
}

func PostProvider(w http.ResponseWriter, r *http.Request) {
	var provider core.Provider
	_ = json.NewDecoder(r.Body).Decode(&provider)
	provider.ID = strconv.Itoa(len(providers) + 1)
	providers = append(providers, provider)
	json.NewEncoder(w).Encode(provider)
	dumpDbToFile(providers, renters)
}

func GetProvider(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range providers {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func PostRenter(w http.ResponseWriter, r *http.Request) {
	var renter core.Renter
	_ = json.NewDecoder(r.Body).Decode(&renter)
	renter.ID = strconv.Itoa(len(renters) + 1)
	renters = append(renters, renter)
	json.NewEncoder(w).Encode(renter)
	dumpDbToFile(providers, renters)
}

func GetRenter(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func GetRenterFiles(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item.Files)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func PostRenterFile(w http.ResponseWriter, r *http.Request) {
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

func GetRenterFile(w http.ResponseWriter, r *http.Request) {
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

func DeleteRenterFile(w http.ResponseWriter, r *http.Request) {
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
