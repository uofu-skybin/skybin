package renter

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os/user"
	"path"
	"skybin/core"
)

func NewServer(renter *Renter, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := &renterServer{
		renter: renter,
		logger: logger,
		router: router,
	}

	router.HandleFunc("/storage", server.postStorage).Methods("POST")
	router.HandleFunc("/files", server.postFiles).Methods("POST")
	router.HandleFunc("/files", server.getFiles).Methods("GET")
	router.HandleFunc("/files/{fileId}/download", server.postDownload).Methods("POST")

	return server
}

type renterServer struct {
	renter *Renter
	logger *log.Logger
	router *mux.Router
}

func (server *renterServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.RemoteAddr, r.Method, r.URL)
	server.router.ServeHTTP(w, r)
}

type postStorageReq struct {
	Amount int64 `json:"amount"`
}

type postStorageResp struct {
	Contracts []*core.Contract `json:"contracts,omitempty"`
	Error     string           `json:"error,omitempty"`
}

func (server *renterServer) postStorage(w http.ResponseWriter, r *http.Request) {
	var req postStorageReq
	var resp postStorageResp

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		server.logger.Println(err)
		resp.Error = err.Error()
		server.writeResp(w, http.StatusBadRequest, &resp)
		return
	}

	contracts, err := server.renter.ReserveStorage(req.Amount)
	if err != nil {
		server.logger.Println(err)
		resp.Error = err.Error()
		server.writeResp(w, http.StatusInternalServerError, &resp)
		return
	}

	resp.Contracts = contracts
	server.writeResp(w, http.StatusCreated, &resp)
}

type postFilesReq struct {
	SourcePath string `json:"sourcePath"`
	DestPath   string `json:"destPath"`
}

type postFilesResp struct {
	File *core.File `json:"file,omitempty"`
	Error string `json:"error,omitempty"`
}

func (server *renterServer) postFiles(w http.ResponseWriter, r *http.Request) {

	var req postFilesReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		server.logger.Println(err)
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	fileInfo, err := server.renter.Upload(req.SourcePath, req.DestPath)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusInternalServerError, &postFilesResp{Error: err.Error()})
		return
	}

	server.writeResp(w, http.StatusCreated, &postFilesResp{File: fileInfo})
}

type getFilesResp struct {
	Files []core.File `json:"files,omitempty"`
	Error string      `json:"error,omitempty"`
}

func (server *renterServer) getFiles(w http.ResponseWriter, r *http.Request) {

	files, err := server.renter.ListFiles()
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusInternalServerError, &getFilesResp{Error: err.Error()})
		return
	}

	server.writeResp(w, http.StatusOK, &getFilesResp{Files: files})
}

// TODO: Update
//func (server *renterServer) getFile(w http.ResponseWriter, r *http.Request) {
//	vars := mux.Vars(r)
//	fileId := vars["fileId"]
//	file, err := server.renter.Lookup(fileId)
//	resp := struct{
//		File *core.File `json:"file"`
//		Error string `json:"error,omitempty"`
//	}{
//		File: file,
//	}
//	if err != nil {
//		server.logger.Println(err)
//		resp.Error = err.Error()
//	}
//	err = json.NewEncoder(w).Encode(&resp)
//	if err != nil {
//		server.logger.Println("error:", err)
//	}
//}

type postDownloadReq struct {
	Destination string `json:"destination,omitempty"`
}

type postDownloadResp struct {
	Error string `json:"error,omitempty"`
}

func (server *renterServer) postDownload(w http.ResponseWriter, r *http.Request) {
	var req postDownloadReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest,
			&postDownloadResp{Error: "bad json"})
		return
	}

	fileId := mux.Vars(r)["fileId"]

	fileInfo, err := server.renter.Lookup(fileId)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest,
			&postDownloadResp{Error: err.Error()})
		return
	}

	// Download to home directory if no destination given
	if len(req.Destination) == 0 {
		user, err := user.Current()
		if err != nil {
			server.logger.Println(err)
			server.writeResp(w, http.StatusInternalServerError,
				&postDownloadResp{Error: err.Error()})
			return
		}
		req.Destination = path.Join(user.HomeDir, fileInfo.Name)
	}

	err = server.renter.Download(fileInfo, req.Destination)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusInternalServerError,
			&postDownloadResp{Error: err.Error()})
		return
	}

	server.writeResp(w, http.StatusCreated, &postDownloadResp{})
}

func (server *renterServer) writeResp(w http.ResponseWriter, status int, body interface{}) {
	w.WriteHeader(status)
	data, err := json.MarshalIndent(body, "", "    ")
	if err != nil {
		server.logger.Fatalf("error: cannot to encode response. error: %s", err)
	}
	_, err = w.Write(data)
	if err != nil {
		server.logger.Fatalf("error: cannot write response body. error: %s", err)
	}
}
