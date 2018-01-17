package metaserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"skybin/core"
	"strconv"

	"github.com/gorilla/mux"
)

// Retrieves the given renter's public RSA key.
func (server *metaServer) getRenterPublicKey(renterID string) (string, error) {
	for _, item := range server.renters {
		if item.ID == renterID {
			return item.PublicKey, nil
		}
	}
	return "", errors.New("could not locate renter with given ID")
}

func (server *metaServer) postRenterHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var renter core.Renter
		_ = json.NewDecoder(r.Body).Decode(&renter)
		renter.ID = strconv.Itoa(len(server.renters) + 1)
		server.renters = append(server.renters, renter)
		json.NewEncoder(w).Encode(renter)
		server.dumpDbToFile(server.providers, server.renters)
	})
}

func (server *metaServer) getRenterHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		for _, item := range server.renters {
			if item.ID == params["id"] {
				json.NewEncoder(w).Encode(item)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

func (server *metaServer) getRenterFilesHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		for _, item := range server.renters {
			if item.ID == params["id"] {
				json.NewEncoder(w).Encode(item.Files)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

func (server *metaServer) postRenterFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		for i, item := range server.renters {
			if item.ID == params["id"] {
				var file core.File
				_ = json.NewDecoder(r.Body).Decode(&file)
				server.renters[i].Files = append(item.Files, file)
				json.NewEncoder(w).Encode(item.Files)
				server.dumpDbToFile(server.providers, server.renters)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

func (server *metaServer) getRenterFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		for _, item := range server.renters {
			if item.ID == params["id"] {
				for _, file := range item.Files {
					if file.ID == params["fileId"] {
						json.NewEncoder(w).Encode(file)
						return
					}
				}
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

func (server *metaServer) deleteRenterFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		for _, item := range server.renters {
			if item.ID == params["id"] {
				for i, file := range item.Files {
					if file.ID == params["fileId"] {
						item.Files = append(item.Files[:i], item.Files[i+1:]...)
						json.NewEncoder(w).Encode(file)
						server.dumpDbToFile(server.providers, server.renters)
						return
					}
				}
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
}
