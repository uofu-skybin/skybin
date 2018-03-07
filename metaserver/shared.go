package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/util"

	"path"

	"github.com/gorilla/mux"
)

func (server *MetaServer) getSharedFilesHandler() http.HandlerFunc {
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

		// Retrieve the renter's shared files.
		files, err := server.db.FindFilesSharedWithRenter(renter.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println(err)
			resp := errorResp{Error: "internal server error"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		for i, _ := range files {
			files[i].Name = path.Base(files[i].Name)
		}
		json.NewEncoder(w).Encode(files)
	})
}

func (server *MetaServer) deleteSharedFileHandler() http.HandlerFunc {
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

		// Remove the file from the renter's directory.
		err = server.db.RemoveFileFromRenterSharedDirectory(params["renterID"], params["fileID"])
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
