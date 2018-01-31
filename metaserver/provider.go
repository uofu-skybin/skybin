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
}

func (server *metaServer) getProvidersHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providers, err := server.db.FindAllProviders()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			resp := errorResp{Error: "internal server error"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		resp := getProvidersResp{
			Providers: providers,
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
		err = server.db.InsertProvider(&provider)
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
			resp := errorResp{Error: "could not find provider"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		json.NewEncoder(w).Encode(provider)
	})
}

func (server *metaServer) putProviderHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		// Make sure provider exists.
		provider, err := server.db.FindProviderByID(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := errorResp{Error: "could not find provider"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Attempt to decode the supplied provider.
		var updatedProvider core.ProviderInfo
		err = json.NewDecoder(r.Body).Decode(&updatedProvider)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			resp := postProviderResp{Error: "could not parse body"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Make sure the user has not changed the provider's ID.
		// BUG(kincaid): Think about other fields users shouldn't change.
		if updatedProvider.ID != provider.ID {
			w.WriteHeader(http.StatusUnauthorized)
			resp := postProviderResp{Error: "must not change provider ID"}
			json.NewEncoder(w).Encode(resp)
			return
		} else if updatedProvider.PublicKey != provider.PublicKey {
			w.WriteHeader(http.StatusUnauthorized)
			resp := postProviderResp{Error: "must not change provider public key"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Put the new provider into the database.
		err = server.db.UpdateProvider(&updatedProvider)
		if err != nil {
			server.logger.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			resp := errorResp{Error: "internal server error"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
		resp := postProviderResp{Provider: updatedProvider}
		json.NewEncoder(w).Encode(resp)
	})
}

func (server *metaServer) deleteProviderHandler() http.HandlerFunc {
	// BUG(kincaid): Validate that the person requesting the data is the specified renter.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		err := server.db.DeleteProvider(params["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			resp := errorResp{Error: "provider not found"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
