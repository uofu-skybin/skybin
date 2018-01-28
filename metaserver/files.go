package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/core"
	"strconv"

	"github.com/gorilla/mux"
)

type fileResp struct {
	FileID core.File `json:"file,omitempty"`
	Error  string    `json:"error,omitempty"`
}

func (server *metaServer) getFilesHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		// Make sure the specified renter actually exists.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Retrieve the renter's files.
		files, err := server.db.FindFilesInRenterDirectory(renter.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println(err)
			return
		}
		json.NewEncoder(w).Encode(files)
	})
}

func (server *metaServer) postFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var file core.File
		err := json.NewDecoder(r.Body).Decode(&file)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}

		params := mux.Vars(r)

		// Make sure the renter exists.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Make sure the file's owner ID is set to that of the renter.
		file.OwnerID = params["renterID"]

		// BUG(kincaid): DB will throw error if file already exists. Might want to check explicitly.
		err = server.db.InsertFile(file)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Insert the file's ID into the renter's directory.
		renter.Files = append(renter.Files, file.ID)
		// BUG(kincaid): Consider trying to roll things back if this fails.
		err = server.db.UpdateRenter(*renter)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(file)
	})
}

func (server *metaServer) getFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			server.logger.Println(err)
			return
		}
		json.NewEncoder(w).Encode(file)
	})
}

func (server *metaServer) deleteFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		// BUG(kincaid): Make sure the renter owns the file they are deleting.
		// Delete the file from the database.
		err := server.db.DeleteFile(params["fileID"])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Remove the file from the renter's directory.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			server.logger.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		removeIndex := -1
		for i, fileID := range renter.Files {
			if fileID == params["fileID"] {
				removeIndex = i
			}
		}
		if removeIndex == -1 {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println("could not find file in renter's directory")
			return
		}
		renter.Files = append(renter.Files[:removeIndex], renter.Files[removeIndex+1:]...)
		err = server.db.UpdateRenter(*renter)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println(err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *metaServer) putFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var file core.File
		err := json.NewDecoder(r.Body).Decode(&file)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: "could not parse payload"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if file.ID != params["fileID"] {
			w.WriteHeader(http.StatusUnauthorized)
			resp := fileResp{Error: "must not change file ID"}
			json.NewEncoder(w).Encode(resp)
			return
		} else if file.OwnerID != params["renterID"] {
			w.WriteHeader(http.StatusUnauthorized)
			resp := fileResp{Error: "must not change file owner"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		err = server.db.UpdateFile(file)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *metaServer) getFileVersionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		versionNum, err := strconv.Atoi(params["version"])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: "must supply int for version"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		file, err := server.db.FindFileByID(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		for _, item := range file.Versions {
			if item.Number == versionNum {
				json.NewEncoder(w).Encode(item)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		resp := fileResp{Error: "could not find specified version"}
		json.NewEncoder(w).Encode(resp)
	})
}

func (server *metaServer) getFileVersionsHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		file, err := server.db.FindFileByID(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		json.NewEncoder(w).Encode(file.Versions)
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
		file, err := server.db.FindFileByID(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		removeIndex := -1
		for i, item := range file.Versions {
			if item.Number == version {
				removeIndex = i
				break
			}
		}
		if removeIndex == -1 {
			w.WriteHeader(http.StatusNotFound)
			resp := fileResp{Error: "could not find specified version"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		file.Versions = append(file.Versions[:removeIndex], file.Versions[removeIndex+1:]...)
		err = server.db.UpdateFile(*file)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *metaServer) postFileVersionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var version core.Version
		err := json.NewDecoder(r.Body).Decode(&version)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: "could not parse payload"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
		}

		// Add 1 for index shift, 1 to get version number
		version.Number = len(file.Versions) + 2
		file.Versions = append(file.Versions, version)

		err = server.db.UpdateFile(*file)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println(err)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func (server *metaServer) putFileVersionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		version, err := strconv.Atoi(params["version"])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: "must supply int for version"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		var newVersion core.Version
		err = json.NewDecoder(r.Body).Decode(&newVersion)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := fileResp{Error: "could not parse payload"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := fileResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
		}

		updateIndex := -1
		for i, item := range file.Versions {
			if item.Number == version {
				updateIndex = i
			}
		}
		if updateIndex == -1 {

		}

		err = server.db.UpdateFile(*file)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println(err)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
