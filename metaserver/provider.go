package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/core"

	"github.com/gorilla/mux"
)

// Retrieves the given provider's public RSA key.
func (server *metaServer) getProviderPublicKey(providerID string) (string, error) {
	provider, err := server.db.FindProviderByID(providerID)
	if err != nil {
		return "", err
	}
	return provider.PublicKey, nil
}

type getProvidersResp struct {
	Providers []core.ProviderInfo `json:"providers"`
	Error     string              `json:"error,omitempty"`
}

func (server *metaServer) getProvidersHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := getProvidersResp{
			Providers: server.db.FindAllProviders(),
		}
		json.NewEncoder(w).Encode(resp)
	})
}

type postProviderResp struct {
	Provider core.ProviderInfo `json:"provider,omitempty"`
	Error    string            `json:"error,omitempty"`
}

func (server *metaServer) postProviderHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var provider core.ProviderInfo
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

		provider.ID = fingerprintKey(provider.PublicKey)
		err = server.db.InsertProvider(provider)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := postProviderResp{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(provider)
	})
}

func (server *metaServer) getProviderHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		provider, err := server.db.FindProviderByID(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(provider)
	})
}
