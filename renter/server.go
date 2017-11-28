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

	router.HandleFunc("/info", server.getInfo).Methods("GET")
	router.HandleFunc("/storage", server.postStorage).Methods("POST")
	router.HandleFunc("/files", server.postFiles).Methods("POST")
	router.HandleFunc("/files", server.getFiles).Methods("GET")
	router.HandleFunc("/files/{fileId}", server.deleteFile).Methods("DELETE")
	router.HandleFunc("/files/{fileId}/download", server.postDownload).Methods("POST")
	router.HandleFunc("/files/{fileId}/permissions", server.postPermissions).Methods("POST")
	router.HandleFunc("/files/shared", server.getSharedFiles).Methods("GET")

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

type errorResp struct {
	Error string `json:"error,omitempty"`
}

func (server *renterServer) getInfo(w http.ResponseWriter, r *http.Request) {
	info, err := server.renter.Info()
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: err.Error()})
		return

	}
	server.writeResp(w, http.StatusOK, info)
}

type postStorageReq struct {
	Amount int64 `json:"amount"`
}

type postStorageResp struct {
	Contracts []*core.Contract `json:"contracts,omitempty"`
}

func (server *renterServer) postStorage(w http.ResponseWriter, r *http.Request) {
	var req postStorageReq
	var resp postStorageResp

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: err.Error()})
		return
	}

	contracts, err := server.renter.ReserveStorage(req.Amount)
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: err.Error()})
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
}

func (server *renterServer) postFiles(w http.ResponseWriter, r *http.Request) {

	var req postFilesReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: "Bad json"})
		return
	}
	if len(req.DestPath) == 0 {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: "No destpath given"})
		return
	}

	var fileInfo *core.File

	// Is this a create folder request?
	if len(req.SourcePath) == 0 {
		fileInfo, err = server.renter.CreateFolder(req.DestPath)
	} else {
		fileInfo, err = server.renter.Upload(req.SourcePath, req.DestPath)
	}
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: err.Error()})
		return
	}

	server.writeResp(w, http.StatusCreated, &postFilesResp{File: fileInfo})
}

type getFilesResp struct {
	Files []*core.File `json:"files,omitempty"`
}

func (server *renterServer) getFiles(w http.ResponseWriter, r *http.Request) {
	files, err := server.renter.ListFiles()
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: err.Error()})
		return
	}

	server.writeResp(w, http.StatusOK, &getFilesResp{Files: files})
}

func (server *renterServer) deleteFile(w http.ResponseWriter, r *http.Request) {
	fileId := mux.Vars(r)["fileId"]
	err := server.renter.Remove(fileId)
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: err.Error()})
		return
	}
	server.writeResp(w, http.StatusOK, &errorResp{})
}

type postDownloadReq struct {
	Destination string `json:"destination,omitempty"`
}

func (server *renterServer) postDownload(w http.ResponseWriter, r *http.Request) {
	var req postDownloadReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: "Bad json"})
		return
	}

	fileId := mux.Vars(r)["fileId"]

	f, err := server.renter.Lookup(fileId)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: err.Error()})
		return
	}

	// Download to home directory if no destination given
	if len(req.Destination) == 0 {
		user, err := user.Current()
		if err != nil {
			server.writeResp(w, http.StatusInternalServerError,
				&errorResp{Error: err.Error()})
			return
		}
		req.Destination = path.Join(user.HomeDir, f.Name)
	}

	err = server.renter.Download(f, req.Destination)
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: err.Error()})
		return
	}

	server.writeResp(w, http.StatusCreated, &errorResp{})
}

type postPermissionsReq struct {
	UserId string `json:"userId"`
}

func (server *renterServer) postPermissions(w http.ResponseWriter, r *http.Request) {
	var req postPermissionsReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: "Bad json"})
		return
	}
	fileId := mux.Vars(r)["fileId"]
	f, err := server.renter.Lookup(fileId)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest,
			&errorResp{Error: err.Error()})
		return
	}
	err = server.renter.ShareFile(f, req.UserId)
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: err.Error()})
		return
	}
	server.writeResp(w, http.StatusCreated, &errorResp{})
}

func (server *renterServer) getSharedFiles(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	server.writeResp(w, http.StatusOK, &getFilesResp{Files: []*core.File{}})
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

	if r, ok := body.(*errorResp); ok && len(r.Error) > 0 {
		server.logger.Print(status, r)
	} else {
		server.logger.Println(status)
	}
}
