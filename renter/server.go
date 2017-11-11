package renter

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"skybin/core"
	"os/user"
	"path"
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
	router.HandleFunc("/files/{fileId}", server.getFile).Methods("GET")
	router.HandleFunc("/files/{fileId}/download", server.postDownload).Methods("POST")

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
		server.logger.Println("error:", err)
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

	fileInfo, err := server.renter.Upload(params.SourcePath, params.DestPath)
	if err != nil {
		server.logger.Println("error:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(&fileInfo)
	if err != nil {
		server.logger.Println(err)
	}
}

func (server *renterServer) getFiles(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("GET", r.URL)
	files, err := server.renter.ListFiles()
	resp := struct{
		Files []core.File `json:"files"`
		Error string `json:"error,omitempty"`
	}{
		Files: files,
	}
	if err != nil {
		server.logger.Println(err)
		resp.Error = err.Error()
		return
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		server.logger.Println("error:", err)
	}
}

func (server *renterServer) getFile(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("GET", r.URL)
	params := mux.Vars(r)
	fileId := params["fileId"]
	file, err := server.renter.Lookup(fileId)
	resp := struct{
		File *core.File `json:"file"`
		Error string `json:"error,omitempty"`
	}{
		File: file,
	}
	if err != nil {
		server.logger.Println(err)
		resp.Error = err.Error()
	}
	err = json.NewEncoder(w).Encode(&resp)
	if err != nil {
		server.logger.Println("error:", err)
	}
}

func (server *renterServer) postDownload(w http.ResponseWriter, r *http.Request) {
	server.logger.Println("POST", r.URL)
	vars := mux.Vars(r)
	fileId := vars["fileId"]
	params := struct{
		Destination string `json:"destination,omitempty"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		server.logger.Println(err)
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	fileInfo, err := server.renter.Lookup(fileId)
	if err != nil {
		server.logger.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Download to home directory if no destination given
	if len(params.Destination) == 0 {
		user, err := user.Current()
		if err != nil {
			server.logger.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		params.Destination = path.Join(user.HomeDir, fileInfo.Name)
	}

	resp := struct{
		Error string `json:"error,omitempty"`
	}{}

	err = server.renter.Download(fileInfo, params.Destination)
	if err != nil {
		server.logger.Println(err)
		resp.Error = err.Error()
	}

	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(&resp)
	if err != nil {
		server.logger.Println(err)
	}
}
