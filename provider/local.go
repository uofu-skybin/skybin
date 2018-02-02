package provider

import (
	"encoding/json"
	"net/http"
	"skybin/core"
)

// This will be used to access statistics to fill out the provider dashboard
func (server *providerServer) getStats(w http.ResponseWriter, r *http.Request) {
	reserved := server.provider.stats.StorageReserved
	used := server.provider.stats.StorageUsed
	free := reserved - used

	info := getInfoResp{
		ProviderId:      server.provider.Config.ProviderID,
		TotalStorage:    1 << 30,
		ReservedStorage: reserved,
		UsedStorage:     used,
		FreeStorage:     free,
		TotalContracts:  len(server.provider.contracts),
	}

	server.writeResp(w, http.StatusOK, &info)
}

// This could potentially be consolidated into the getStats object
func (server *providerServer) getContracts(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK,
		getContractsResp{Contracts: server.provider.contracts})
}

// This could potentially be consolidated into the getStats object
func (server *providerServer) getActivity(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK, &getActivityResp{Activity: server.provider.activity})
}

func (server *providerServer) postInfo(w http.ResponseWriter, r *http.Request) {
	var params core.ProviderInfo
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest, &errorResp{"Bad json"})
	}
	// server.provider.Config = &params
	server.writeResp(w, http.StatusOK, &errorResp{})
}

type getInfoResp struct {
	ProviderId      string `json:"providerId"`
	TotalStorage    int64  `json:"providerAllocated"`
	ReservedStorage int64  `json:"providerReserved"`
	UsedStorage     int64  `json:"providerUsed"`
	FreeStorage     int64  `json:"providerFree"`
	TotalContracts  int    `json:"providerContracts"`
}
type getActivityResp struct {
	Activity []Activity `json:"activity"`
}
