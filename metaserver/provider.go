package metaserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"skybin/core"
	"strconv"

	"github.com/gorilla/mux"
)

// Retrieves the given provider's public RSA key.
func (server *metaServer) getProviderPublicKey(providerID string) (string, error) {
	for _, item := range server.providers {
		if item.ID == providerID {
			return item.PublicKey, nil
		}
	}
	return "", errors.New("could not locate provider with given ID")
}

type getProvidersResp struct {
	Providers []core.Provider `json:"providers"`
	Error     string          `json:"error,omitempty"`
}

func (server *metaServer) getProvidersHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := getProvidersResp{
			Providers: server.providers,
		}
		json.NewEncoder(w).Encode(resp)
	})
}

type postProviderResp struct {
	Provider core.Provider `json:"provider"`
	Error    string        `json:"error,omitempty"`
}

func (server *metaServer) postProviderHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var provider core.Provider
		_ = json.NewDecoder(r.Body).Decode(&provider)
		provider.ID = strconv.Itoa(len(server.providers) + 1)
		server.providers = append(server.providers, provider)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(provider)
		server.dumpDbToFile(server.providers, server.renters)
	})
}

func (server *metaServer) getProviderHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		for _, item := range server.providers {
			if item.ID == params["id"] {
				json.NewEncoder(w).Encode(item)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
}
