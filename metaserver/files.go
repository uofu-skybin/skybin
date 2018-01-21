package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/core"
	"strconv"

	"github.com/gorilla/mux"
)

type fileResp struct {
	File  core.File `json:"file,omitempty"`
	Error string    `json:"error,omitempty"`
}

func (server *metaServer) postFileHandler() http.HandlerFunc {
	// BUG(kincaid): Validate that the file's owner id matches the user's
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var file core.File
		err := json.NewDecoder(r.Body).Decode(&file)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: "could not parse body"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		err = server.db.InsertFile(file)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		json.NewEncoder(w).Encode(file)
	})
}

func (server *metaServer) getFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		file, err := server.db.FindFileByID(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(file)
	})
}

func (server *metaServer) getFileVersionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		version, err := strconv.Atoi(params["version"])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: "must supply int for version"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		file, err := server.db.FindFileByIDAndVersion(params["id"], version)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(file)
	})
}

func (server *metaServer) deleteFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		err := server.db.DeleteFile(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *metaServer) deleteFileVersionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		version, err := strconv.Atoi(params["version"])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: "must supply int for version"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		err = server.db.DeleteFileVersion(params["id"], version)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
