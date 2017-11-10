package main

import (
	"log"
	"path"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"encoding/json"
)

var renterCmd = Cmd{
	Name: "renter",
	Description: "Start a renter server",
	Run: runRenter,
}

type RenterConfig struct {
	Addr string `json:"address"`
	MetaAddr string `json:"metadataServiceAddress"`
}

type renterAPI struct {
	config *RenterConfig
	logger *log.Logger
}

func (api *renterAPI) postStorage(w http.ResponseWriter, r *http.Request) {
	api.logger.Println("POST", r.URL)
	params := struct{
		Amount int `json:"amount"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

}

func (api *renterAPI) postFiles(w http.ResponseWriter, r *http.Request) {
	api.logger.Println("POST", r.URL)
	params := struct{
		SourcePath string `json:"sourcePath"`
		DestPath string `json:"destPath"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
}

func (api *renterAPI) getFiles(w http.ResponseWriter, r *http.Request) {
	api.logger.Println("GET", r.URL)
}

func (api *renterAPI) getFile(w http.ResponseWriter, r *http.Request) {
	api.logger.Println("GET", r.URL)
	params := mux.Vars(r)
	_, exists := params["filename"]
	if !exists{
		log.Fatal("no filename param")
	}

}

func (api *renterAPI) postDownload(w http.ResponseWriter, r *http.Request) {
	api.logger.Println("POST", r.URL)
	params := mux.Vars(r)
	_, exists := params["filename"]
	if !exists {
		log.Fatal("no filename param")
	}
}

func runRenter(args ...string) {

	homedir, err := findHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	var config RenterConfig
	err = loadJSON(path.Join(homedir, "renter", "config.json"), &config)
	if err != nil {
		log.Fatal(err)
	}

	api := renterAPI{
		config: &config,
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}

	router := mux.NewRouter()

	router.HandleFunc("/storage", api.postStorage).Methods("POST")
	router.HandleFunc("/files", api.postFiles).Methods("POST")
	router.HandleFunc("/files", api.getFiles).Methods("GET")
	router.HandleFunc("/files/{filename}", api.getFile).Methods("GET")
	router.HandleFunc("/files/{filename}/download", api.postDownload).Methods("POST")

	log.Println("starting renter service at", config.Addr)
	log.Fatal(http.ListenAndServe(config.Addr, router))
}
