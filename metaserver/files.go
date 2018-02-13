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

func (server *MetaServer) getFilesHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		// Make sure the specified renter actually exists.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		// Retrieve the renter's files.
		files, err := server.db.FindFilesInRenterDirectory(renter.ID)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		json.NewEncoder(w).Encode(files)
	})
}

func (server *MetaServer) postFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var file core.File
		err := json.NewDecoder(r.Body).Decode(&file)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		params := mux.Vars(r)

		// Make sure the renter exists.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the file's owner ID is set to that of the renter.
		file.OwnerID = params["renterID"]

		// BUG(kincaid): DB will throw error if file already exists. Might want to check explicitly.
		err = server.db.InsertFile(&file)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Insert the file's ID into the renter's directory.
		err = server.db.AddFileToRenterDirectory(renter.ID, file.ID)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(file)
	})
}

func (server *MetaServer) getFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		json.NewEncoder(w).Encode(file)
	})
}

func (server *MetaServer) deleteFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		// BUG(kincaid): Make sure the renter owns the file they are deleting.
		// Delete the file from the database.
		err := server.db.DeleteFile(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}
		// Remove the file from the renter's directory.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		err = server.db.RemoveFileFromRenterDirectory(renter.ID, params["fileID"])
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *MetaServer) putFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var file core.File
		err := json.NewDecoder(r.Body).Decode(&file)
		if err != nil {
			writeErr("could not parse payload", http.StatusBadRequest, w)
			return
		}

		if file.ID != params["fileID"] {
			writeErr("must not change file ID", http.StatusUnauthorized, w)
			return
		} else if file.OwnerID != params["renterID"] {
			writeErr("must not change file owner", http.StatusUnauthorized, w)
			return
		}

		err = server.db.UpdateFile(&file)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *MetaServer) getFileVersionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		versionNum, err := strconv.Atoi(params["version"])
		if err != nil {
			writeErr("must supply int for version", http.StatusBadRequest, w)
			return
		}
		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		for _, item := range file.Versions {
			if item.Num == versionNum {
				json.NewEncoder(w).Encode(item)
				return
			}
		}
		writeErr("could not find specified version", http.StatusNotFound, w)
	})
}

func (server *MetaServer) getFileVersionsHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		json.NewEncoder(w).Encode(file.Versions)
	})
}

func (server *MetaServer) deleteFileVersionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		version, err := strconv.Atoi(params["version"])
		if err != nil {
			writeErr("must supply int for version", http.StatusBadRequest, w)
			return
		}

		err = server.db.DeleteFileVersion(params["fileID"], version)
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *MetaServer) postFileVersionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var version core.Version
		err := json.NewDecoder(r.Body).Decode(&version)
		if err != nil {
			writeErr("could not parse payload", http.StatusBadRequest, w)
			return
		}

		err = server.db.InsertFileVersion(params["fileID"], &version)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		w.WriteHeader(http.StatusCreated)
	})
}

func (server *MetaServer) putFileVersionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		version, err := strconv.Atoi(params["version"])
		if err != nil {
			writeErr("must supply int for version", http.StatusNotFound, w)
			return
		}

		var newVersion core.Version
		err = json.NewDecoder(r.Body).Decode(&newVersion)
		if err != nil {
			writeErr("could not parse payload", http.StatusBadRequest, w)
			return
		}

		if version != newVersion.Num {
			writeErr("must not update version number", http.StatusBadRequest, w)
			return
		}

		err = server.db.UpdateFileVersion(params["fileID"], &newVersion)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func (server *MetaServer) getFilePermissionsHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)
		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		json.NewEncoder(w).Encode(file.AccessList)
	})
}

func (server *MetaServer) getFilePermissionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		for _, item := range file.AccessList {
			if item.RenterId == params["sharedID"] {
				json.NewEncoder(w).Encode(item)
				return
			}
		}
		writeErr("could not find specified permission", http.StatusNotFound, w)

	})
}

func (server *MetaServer) postFilePermissionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var permission core.Permission
		err := json.NewDecoder(r.Body).Decode(&permission)
		if err != nil {
			writeErr("could not parse payload", http.StatusBadRequest, w)
			return
		}

		// Make sure a valid renter is specified in the permission.
		_, err = server.db.FindRenterByID(permission.RenterId)
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}

		// Add the permission to the ACL
		err = server.db.AddPermissionToFileACL(params["fileID"], &permission)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Add the file to the renter's directory
		err = server.db.AddFileToRenterSharedDirectory(permission.RenterId, params["fileID"])
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		w.WriteHeader(http.StatusCreated)
	})
}

func (server *MetaServer) putFilePermissionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var newPermission core.Permission
		err := json.NewDecoder(r.Body).Decode(&newPermission)
		if err != nil {
			writeErr("could not parse payload", http.StatusBadRequest, w)
			return
		}

		err = server.db.UpdateFilePermission(params["fileID"], &newPermission)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func (server *MetaServer) deleteFilePermissionHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BUG(kincaid): Validate that the file's owner id matches the user's or the user is in the file's ACL
		params := mux.Vars(r)

		// Remove the user from the file's ACL
		err := server.db.RemoveFilePermissionFromACL(params["fileID"], params["sharedID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Remove the file from the sharee's directory
		err = server.db.RemoveFileFromRenterSharedDirectory(params["sharedID"], params["fileID"])
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
