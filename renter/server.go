package renter

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func NewServer(renter *Renter, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := renterServer{
		renter: renter,
		logger: logger,
		router: router,
	}

	router.HandleFunc("/storage", server.postStorage).Methods("POST")
	router.HandleFunc("/files", server.postFiles).Methods("POST")
	router.HandleFunc("/files", server.getFiles).Methods("GET")
	router.HandleFunc("/files/{filename}", server.getFile).Methods("GET")
	router.HandleFunc("/files/{filename}/download", server.postDownload).Methods("POST")

	return router
}

type renterServer struct {
	renter *Renter
	logger *log.Logger
	router *mux.Router
}

func (server *renterServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.router.ServeHTTP(w, r)
}

func (server *renterServer) postStorage(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("POST", r.URL)
	params := struct {
		Amount int64 `json:"amount"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	err = server.renter.ReserveStorage(params.Amount)
	if err != nil {
		server.logger.Println("error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (server *renterServer) postFiles(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("POST", r.URL)
	params := struct {
		SourcePath string `json:"sourcePath"`
		DestPath   string `json:"destPath"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	err = server.renter.Upload(params.SourcePath, params.DestPath)
	if err != nil {
		server.logger.Println("error:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (server *renterServer) getFiles(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("GET", r.URL)
}

func (server *renterServer) getFile(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("GET", r.URL)
	params := mux.Vars(r)
	_, exists := params["filename"]
	if !exists {
		log.Fatal("no filename param")
	}

}

func (server *renterServer) postDownload(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("POST", r.URL)
	params := mux.Vars(r)
	_, exists := params["filename"]
	if !exists {
		log.Fatal("no filename param")
	}
}
