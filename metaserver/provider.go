package metaserver

import (
	"encoding/json"
	"net/http"
	"skybin/core"

	"crypto/rsa"
	"skybin/util"

	"github.com/gorilla/mux"
)

// Retrieves the given provider's public RSA key.
func (server *MetaServer) getProviderPublicKey(providerID string) (*rsa.PublicKey, error) {
	provider, err := server.db.FindProviderByID(providerID)
	if err != nil {
		return nil, err
	}
	key, err := util.UnmarshalPublicKey([]byte(provider.PublicKey))
	if err != nil {
		return nil, err
	}
	return key, nil
}

type getProvidersResp struct {
	Providers []core.ProviderInfo `json:"providers"`
}

func (server *MetaServer) getProvidersHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providers, err := server.db.FindAllProviders()
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
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

func (server *MetaServer) postProviderHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var provider core.ProviderInfo
		err := json.NewDecoder(r.Body).Decode(&provider)

		if err != nil {
			writeErr("unable to parse payload", http.StatusBadRequest, w)
			return
		}

		// Make sure the user supplied a public key for the provider.
		if provider.PublicKey == "" {
			writeErr("must specify RSA public key", http.StatusBadRequest, w)
			return
		}

		_, err = parsePublicKey(provider.PublicKey)
		if err != nil {
			writeErr("invalid RSA public key", http.StatusBadRequest, w)
			return
		}

		provider.ID = fingerprintKey(provider.PublicKey)
		err = server.db.InsertProvider(&provider)
		if err != nil {
			writeErr(err.Error(), http.StatusBadRequest, w)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(provider)
	})
}

func (server *MetaServer) getProviderHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		provider, err := server.db.FindProviderByID(params["id"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		json.NewEncoder(w).Encode(provider)
	})
}

func (server *MetaServer) putProviderHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		// Make sure provider exists.
		provider, err := server.db.FindProviderByID(params["id"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		// Attempt to decode the supplied provider.
		var updatedProvider core.ProviderInfo
		err = json.NewDecoder(r.Body).Decode(&updatedProvider)
		if err != nil {
			writeErr("could not parse payload", http.StatusBadRequest, w)
			return
		}
		// Make sure the user has not changed the provider's ID.
		// BUG(kincaid): Think about other fields users shouldn't change.
		if updatedProvider.ID != provider.ID {
			writeErr("must not change provider ID", http.StatusUnauthorized, w)
			return
		} else if updatedProvider.PublicKey != provider.PublicKey {
			writeErr("must not change provider public key", http.StatusBadRequest, w)
			return
		}
		// Put the new provider into the database.
		err = server.db.UpdateProvider(&updatedProvider)
		if err != nil {
			writeAndLogInternalError(err, w, server.logger)
			return
		}
		w.WriteHeader(http.StatusOK)
		resp := postProviderResp{Provider: updatedProvider}
		json.NewEncoder(w).Encode(resp)
	})
}

func (server *MetaServer) deleteProviderHandler() http.HandlerFunc {
	// BUG(kincaid): Validate that the person requesting the data is the specified renter.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		err := server.db.DeleteProvider(params["id"])
		if err != nil {
			writeErr(err.Error(), http.StatusNotFound, w)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
