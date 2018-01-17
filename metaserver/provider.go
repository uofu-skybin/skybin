package metaserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"skybin/core"

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
	Provider core.Provider `json:"provider,omitempty"`
	Error    string        `json:"error,omitempty"`
}

func (server *metaServer) postProviderHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var provider core.Provider
		err := json.NewDecoder(r.Body).Decode(&provider)

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := postProviderResp{Error: "unable to parse payload"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Make sure the user supplied a public key for the provider.
		if provider.PublicKey == "" {
			w.WriteHeader(http.StatusBadRequest)
			resp := postProviderResp{Error: "must specify RSA public key"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		_, err = parsePublicKey(provider.PublicKey)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := postProviderResp{Error: "invalid RSA public key"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		provider.ID, err = fingerprintKey(provider.PublicKey)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := postProviderResp{Error: "could not generate ID from supplied public key"}
			json.NewEncoder(w).Encode(resp)
			return
		}
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
