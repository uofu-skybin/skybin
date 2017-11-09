package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type Provider struct {
	ID          string `json:id,omitempty`
	PublicKey   string `json:publicKey,omitempty`
	Host        string `json:host,omitempty`
	Port        int    `json:port,omitempty`
	SpaceAvail  int    `json:spaceAvail,omitempty`
	StorageRate int    `json:storageRate,omitempty`
}

type Block struct {
	ID        string     `json:id,omitempty`
	Locations []Provider `json:locations,omitempty`
}

type File struct {
	ID     string  `json:id,omitempty`
	Name   string  `json:name,omitempty`
	Blocks []Block `json:blocks,omitempty`
}

type Renter struct {
	ID        string `json:id,omitempty`
	PublicKey string `json:publicKey,omitempty`
	Files     []File `json:files,omitempty`
}

var providers []Provider
var renters []Renter

// our main function
func main() {
	router := mux.NewRouter()

	providers = append(providers, Provider{ID: "1", PublicKey: "test", Host: "test", Port: 2, SpaceAvail: 50, StorageRate: 5})

	router.HandleFunc("/providers", GetProviders).Methods("GET")
	router.HandleFunc("/providers", PostProvider).Methods("POST")
	router.HandleFunc("/providers/{id}", GetProvider).Methods("GET")

	router.HandleFunc("/renters", PostRenter).Methods("POST")
	router.HandleFunc("/renters/{id}", GetRenter).Methods("GET")
	router.HandleFunc("/renters/{id}/files", GetRenterFiles).Methods("GET")
	router.HandleFunc("/renters/{id}/files", PostRenterFile).Methods("POST")
	router.HandleFunc("/renters/{id}/files/{fileId}", GetRenterFile).Methods("GET")
	router.HandleFunc("/renters/{id}/files/{fileId}", DeleteRenterFile).Methods("DELETE")

	log.Fatal(http.ListenAndServe(":8000", router))
}

func GetProviders(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(providers)
}

func PostProvider(w http.ResponseWriter, r *http.Request) {
	var provider Provider
	_ = json.NewDecoder(r.Body).Decode(&provider)
	provider.ID = strconv.Itoa(len(providers) + 1)
	providers = append(providers, provider)
	json.NewEncoder(w).Encode(provider)
}

func GetProvider(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range providers {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	json.NewEncoder(w).Encode(&Provider{})
}

func PostRenter(w http.ResponseWriter, r *http.Request) {
	var renter Renter
	_ = json.NewDecoder(r.Body).Decode(&renter)
	renter.ID = strconv.Itoa(len(renters) + 1)
	renters = append(renters, renter)
	json.NewEncoder(w).Encode(renter)
}

func GetRenter(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	json.NewEncoder(w).Encode(&Renter{})
}

func GetRenterFiles(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item.Files)
			return
		}
	}
	var empty []File
	json.NewEncoder(w).Encode(&empty)
}

func PostRenterFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			var file File
			_ = json.NewDecoder(r.Body).Decode(&file)
			item.Files = append(item.Files, file)
			json.NewEncoder(w).Encode(item.Files)
			return
		}
	}
	var empty []File
	json.NewEncoder(w).Encode(&empty)
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
	json.NewEncoder(w).Encode(&File{})
}

func DeleteRenterFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			for i, file := range item.Files {
				if file.ID == params["fileId"] {
					item.Files = append(item.Files[:i], item.Files[i+1:]...)
					json.NewEncoder(w).Encode(file)
					return
				}
			}
		}
	}
	json.NewEncoder(w).Encode(&File{})
}
