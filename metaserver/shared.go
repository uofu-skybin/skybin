package metaserver

import (
	"encoding/json"
	"net/http"

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
		// Retrieve the renter's shared files.
		files, err := server.db.FindFilesSharedWithRenter(renter.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println(err)
			resp := errorResp{Error: "internal server error"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		json.NewEncoder(w).Encode(files)
	})
}

func (server *MetaServer) deleteSharedFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		// Remove the file from the renter's directory.
		renter, err := server.db.FindRenterByID(params["renterID"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := errorResp{Error: "could not find renter"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		removeIndex := -1
		for i, fileID := range renter.Shared {
			if fileID == params["fileID"] {
				removeIndex = i
			}
		}
		if removeIndex == -1 {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println("could not find file in renter's shared directory")
			resp := errorResp{Error: "internal server error"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		renter.Shared = append(renter.Shared[:removeIndex], renter.Shared[removeIndex+1:]...)
		err = server.db.UpdateRenter(renter)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			server.logger.Println(err)
			resp := errorResp{Error: "internal server error"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
