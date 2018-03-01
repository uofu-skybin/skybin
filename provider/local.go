package provider

import (
	"encoding/json"

	"log"
	"net/http"
	"path"
	"skybin/core"
	"skybin/util"

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
	router.HandleFunc("/config", server.getConfig).Methods("GET")
	router.HandleFunc("/config", server.postConfig).Methods("POST")
	router.HandleFunc("/info", server.getInfo).Methods("GET")
	router.HandleFunc("/activity", server.getActivity).Methods("GET")
	router.HandleFunc("/contracts", server.getContracts).Methods("GET")
	router.HandleFunc("/stats", server.getStats).Methods("GET")

	return &server
}

func (server *localServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
	server.router.ServeHTTP(w, r)
}

func (server *localServer) getInfo(w http.ResponseWriter, r *http.Request) {
	info := server.provider.GetInfo()
	server.writeResp(w, http.StatusOK, info)
}

func (server *localServer) getConfig(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK, server.provider.Config)
}

type getContractsResp struct {
	Contracts []*core.Contract `json:"contracts"`
}

func (server *localServer) getContracts(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK,
		getContractsResp{Contracts: server.provider.contracts})
}

func (server *localServer) getActivity(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK, &getActivityResp{Activity: server.provider.activity})
}

// This could potentially replace getActivity (which is not currently used)
func (server *localServer) getStats(w http.ResponseWriter, r *http.Request) {
	// don't change any metrics but cycle data as needed
	server.provider.addActivity("update", 0)
	server.writeResp(w, http.StatusOK, server.provider.stats)
}

func (server *localServer) postConfig(w http.ResponseWriter, r *http.Request) {
	var params Config
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest, &errorResp{"Bad json"})
		return
	}

	server.provider.Config.SpaceAvail = params.SpaceAvail
	server.provider.Config.StorageRate = params.StorageRate
	server.provider.Config.PublicApiAddr = params.PublicApiAddr

	err = util.SaveJson(path.Join(server.provider.Homedir, "config.json"), &server.provider.Config)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest, errorResp{Error: "Error saving config file"})
		return
	}

	err = server.provider.saveSnapshot()
	if err != nil {
		server.logger.Println("Unable to save snapshot. Error:", err)
		return
	}

	server.writeResp(w, http.StatusOK, &errorResp{})
}

type getActivityResp struct {
	Activity []Activity `json:"activity"`
}

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
