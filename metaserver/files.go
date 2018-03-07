package metaserver

import (
	"encoding/json"
	"net/http"
	"path"
	"skybin/core"
	"skybin/util"
	"strconv"

	"github.com/dgrijalva/jwt-go"

	"github.com/gorilla/mux"
)

type fileResp struct {
	FileID core.File `json:"file,omitempty"`
	Error  string    `json:"error,omitempty"`
}

func userOwnsFile(file *core.File, claims jwt.MapClaims) bool {
	renterID, present := claims["renterID"]
	if !present {
		return false
	}
	return renterID.(string) == file.OwnerID
}

func canAccessFile(file *core.File, claims jwt.MapClaims) bool {
	renterID, present := claims["renterID"]
	if !present {
		return false
	}
	if renterID.(string) == file.OwnerID {
		return true
	}
	for _, permission := range file.AccessList {
		if permission.RenterId == renterID.(string) {
			return true
		}
	}
	return false
}

func (server *MetaServer) getFilesHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != params["renterID"] {
			writeErr("cannot access other users' files", http.StatusUnauthorized, w)
			return
		}

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
		params := mux.Vars(r)

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != params["renterID"] {
			writeErr("cannot access other users' files", http.StatusUnauthorized, w)
			return
		}

		var file core.File
		err = json.NewDecoder(r.Body).Decode(&file)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

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

		// Make sure the person making the request is either the renter who owns the file or present in the file's ACL.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		if !canAccessFile(file, claims) {
			writeErr("not authorized to access file", http.StatusUnauthorized, w)
			return
		}
		if !userOwnsFile(file, claims) {
			file.Name = path.Base(file.Name)
		}

		json.NewEncoder(w).Encode(file)
	})
}

func (server *MetaServer) deleteFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the file.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != file.OwnerID {
			writeErr("cannot delete other users' files", http.StatusUnauthorized, w)
			return
		}

		// If the file is a folder, make sure to remove children as well.
		var removed []core.File
		if file.IsDir {
			removed, err = server.db.RemoveFolderChildren(file)
			if err != nil {
				writeErr(err.Error(), http.StatusBadRequest, w)
				return
			}
		}

		err = server.db.DeleteFile(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}
		removed = append(removed, *file)

		// Remove the files from the renter's directory.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		for _, f := range removed {
			err = server.db.RemoveFileFromRenterDirectory(renter.ID, f.ID)
			if err != nil {
				writeAndLogInternalError(err, w, server.logger)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (server *MetaServer) putFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		var newFile core.File
		err := json.NewDecoder(r.Body).Decode(&newFile)
		if err != nil {
			writeErr("could not parse payload", http.StatusBadRequest, w)
			return
		}

		oldFile, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		if newFile.ID != oldFile.ID {
			writeErr("must not change file ID", http.StatusUnauthorized, w)
			return
		} else if newFile.OwnerID != oldFile.OwnerID {
			writeErr("must not change file owner", http.StatusUnauthorized, w)
			return
		} else if newFile.IsDir != oldFile.IsDir {
			writeErr("must not change whether or not file is directory", http.StatusUnauthorized, w)
			return
		}

		// Make sure the person making the request is the renter who owns the files.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != newFile.OwnerID {
			writeErr("cannot modify other users' files", http.StatusUnauthorized, w)
			return
		}

		// If the file is a directory and we're changing its name, make sure we update its children.
		if oldFile.IsDir && newFile.Name != oldFile.Name {
			err = server.db.RenameFolder(oldFile.ID, oldFile.OwnerID, oldFile.Name, newFile.Name)
			if err != nil {
				writeErr(err.Error(), http.StatusBadRequest, w)
				return
			}
		}

		err = server.db.UpdateFile(&newFile)
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

		// Make sure the person making the request is either the renter who owns the file or present in the file's ACL.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		if !canAccessFile(file, claims) {
			writeErr("not authorized to access file", http.StatusUnauthorized, w)
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

		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}

		if !canAccessFile(file, claims) {
			writeErr("not authorized to access file", http.StatusUnauthorized, w)
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

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the file.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != file.OwnerID {
			writeErr("cannot delete other users' versions", http.StatusUnauthorized, w)
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

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the file.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != file.OwnerID {
			writeErr("cannot delete other users' versions", http.StatusUnauthorized, w)
			return
		}

		var version core.Version
		err = json.NewDecoder(r.Body).Decode(&version)
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

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the file.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != file.OwnerID {
			writeErr("cannot delete other users' versions", http.StatusUnauthorized, w)
			return
		}

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

		// Make sure the person making the request is the renter who owns the file.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if !canAccessFile(file, claims) {
			writeErr("not authorized to access file", http.StatusUnauthorized, w)
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

		// Make sure the person making the request is the renter who owns the file.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if !canAccessFile(file, claims) {
			writeErr("not authorized to access file", http.StatusUnauthorized, w)
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

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the file.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != file.OwnerID {
			writeErr("cannot share other users' files", http.StatusUnauthorized, w)
			return
		}

		var permission core.Permission
		err = json.NewDecoder(r.Body).Decode(&permission)
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

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the file.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != file.OwnerID {
			writeErr("cannot modify other users' files", http.StatusUnauthorized, w)
			return
		}

		var newPermission core.Permission
		err = json.NewDecoder(r.Body).Decode(&newPermission)
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

		file, err := server.db.FindFileByID(params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		// Make sure the person making the request is the renter who owns the file.
		claims, err := util.GetTokenClaimsFromRequest(r)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		if renterID, present := claims["renterID"]; !present || renterID.(string) != file.OwnerID {
			writeErr("cannot unshare other users' files", http.StatusUnauthorized, w)
			return
		}

		// Remove the user from the file's ACL
		err = server.db.RemoveFilePermissionFromACL(params["fileID"], params["sharedID"])
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
