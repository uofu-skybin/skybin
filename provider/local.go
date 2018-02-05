package provider

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type localServer struct {
	provider *Provider
	logger   *log.Logger
	router   *mux.Router
}

func NewLocalServer(provider *Provider, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := localServer{
		provider: provider,
		logger:   logger,
		router:   router,
	}

	router.HandleFunc("/info", server.postInfo).Methods("POST")
	router.HandleFunc("/stats", server.getStats).Methods("GET")
	router.HandleFunc("/activity", server.getActivity).Methods("GET")
	router.HandleFunc("/contracts", server.getContracts).Methods("GET")

	return &server
}

func (server *localServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
	server.router.ServeHTTP(w, r)
}

// This will be used to access statistics to fill out the provider dashboard
func (server *localServer) getStats(w http.ResponseWriter, r *http.Request) {
	reserved := server.provider.stats.StorageReserved
	used := server.provider.stats.StorageUsed
	free := reserved - used

	info := getInfoResp{
		ProviderId:      server.provider.Config.ProviderID,
		TotalStorage:    server.provider.Config.SpaceAvail,
		ReservedStorage: reserved,
		UsedStorage:     used, //maybe delete these fields
		FreeStorage:     free, //maybe delete these fields
		TotalContracts:  len(server.provider.contracts),
	}

	server.writeResp(w, http.StatusOK, &info)
}

// This could potentially be consolidated into the getStats object
func (server *localServer) getContracts(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK,
		getContractsResp{Contracts: server.provider.contracts})
}

// This could potentially be consolidated into the getStats object
func (server *localServer) getActivity(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK, &getActivityResp{Activity: server.provider.activity})
}

// Info object for when a provider saves in UI
func (server *localServer) postInfo(w http.ResponseWriter, r *http.Request) {
	var params Config
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest, &errorResp{"Bad json"})
	}

	server.provider.Config.SpaceAvail = params.SpaceAvail
	server.provider.Config.StorageRate = params.StorageRate
	server.provider.Config.ApiAddr = params.ApiAddr

	err = server.provider.UpdateMeta()
	if err != nil {
		server.writeResp(w, http.StatusBadRequest, &errorResp{"Error updating metadata server."})
	}

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

//TODO: refactor this into a single function between local and public???
func (server *localServer) writeResp(w http.ResponseWriter, status int, body interface{}) {
	w.WriteHeader(status)
	data, err := json.MarshalIndent(body, "", "    ")
	if err != nil {
		server.logger.Fatalf("error: cannot to encode response. error: %s", err)
	}
	_, err = w.Write(data)
	if err != nil {
		server.logger.Fatalf("error: cannot write response body. error: %s", err)
	}

	if r, ok := body.(*errorResp); ok && len(r.Error) > 0 {
		server.logger.Print(status, r)
	} else {
		server.logger.Println(status)
	}
}
