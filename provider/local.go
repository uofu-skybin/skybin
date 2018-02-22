package provider

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"skybin/core"

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

	router.HandleFunc("/config", server.postConfig).Methods("POST")
	router.HandleFunc("/info", server.getInfo).Methods("GET")
	router.HandleFunc("/activity", server.getActivity).Methods("GET")
	router.HandleFunc("/contracts", server.getContracts).Methods("GET")

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

func (server *localServer) postConfig(w http.ResponseWriter, r *http.Request) {
	var params Config
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest, &errorResp{"Bad json"})
	}

	server.provider.Config.SpaceAvail = params.SpaceAvail
	server.provider.Config.StorageRate = params.StorageRate
	server.provider.Config.PublicApiAddr = params.PublicApiAddr

	err = server.provider.UpdateMeta()
	if err != nil {
		msg := fmt.Sprintf("Error updating metadata server: %s", err)
		server.writeResp(w, http.StatusBadRequest, &errorResp{msg})
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
