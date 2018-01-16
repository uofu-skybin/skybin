package metaserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"skybin/core"
	"strconv"

	"github.com/gorilla/mux"
)

var providers []core.Provider

func getProviderPublicKey(providerID string) (string, error) {
	for _, item := range providers {
		if item.ID == providerID {
			return item.PublicKey, nil
		}
	}
	return "", errors.New("Could not locate provider with given ID.")
}

type getProvidersResp struct {
	Providers []core.Provider `json:"providers"`
	Error     string          `json:"error,omitempty"`
}

var getProvidersHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := getProvidersResp{
		Providers: providers,
	}
	json.NewEncoder(w).Encode(resp)
})

type postProviderResp struct {
	Provider core.Provider `json:"provider"`
	Error    string        `json:"error,omitempty"`
}

var postProviderHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	var provider core.Provider
	_ = json.NewDecoder(r.Body).Decode(&provider)
	provider.ID = strconv.Itoa(len(providers) + 1)
	providers = append(providers, provider)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(provider)
	dumpDbToFile(providers, renters)
})

var getProviderHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range providers {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
})
