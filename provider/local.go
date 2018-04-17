package provider

import (
	"encoding/json"
	"fmt"

	"log"
	"net/http"
	"skybin/core"
	"skybin/metaserver"
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
	router.HandleFunc("/private-info", server.getPrivateInfo).Methods("GET")
	router.HandleFunc("/contracts", server.getContracts).Methods("GET")
	router.HandleFunc("/stats", server.getStats).Methods("GET")
	router.HandleFunc("/paypal/withdraw", server.withdraw).Methods("POST")
	router.HandleFunc("/transactions", server.getTransactions).Methods("GET")

	return &server
}

func (server *localServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
	server.router.ServeHTTP(w, r)
}

func (server *localServer) getInfo(w http.ResponseWriter, r *http.Request) {
	info := server.provider.GetPublicInfo()
	server.writeResp(w, http.StatusOK, info)
}

func (server *localServer) getPrivateInfo(w http.ResponseWriter, r *http.Request) {
	info, err := server.provider.GetPrivateInfo()
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError, &errorResp{err.Error()})
		return
	}
	server.writeResp(w, http.StatusOK, info)
}

func (server *localServer) getConfig(w http.ResponseWriter, r *http.Request) {
	server.writeResp(w, http.StatusOK, server.provider.Config)
}

type getContractsResp struct {
	Contracts []*core.Contract `json:"contracts"`
}

// This is currently not being used anywhere in the frontend
func (server *localServer) getContracts(w http.ResponseWriter, r *http.Request) {
	contracts, err := server.provider.db.GetAllContracts()
	if err != nil {
		msg := fmt.Sprintf("Error retrieving contract list: %s", err)
		server.writeResp(w, http.StatusInternalServerError, &errorResp{msg})
	}
	server.writeResp(w, http.StatusOK, &getContractsResp{Contracts: contracts})
}

func (server *localServer) getStats(w http.ResponseWriter, r *http.Request) {
	resp, err := server.provider.db.GetActivityStats()
	if err != nil {
		msg := fmt.Sprintf("Failed to make stats response: %s", err)
		server.writeResp(w, http.StatusInternalServerError, &errorResp{msg})
		return
	}
	server.writeResp(w, http.StatusOK, resp)
}

func (server *localServer) postConfig(w http.ResponseWriter, r *http.Request) {
	var params Config
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		server.writeResp(w, http.StatusBadRequest, &errorResp{"Bad json"})
		return
	}

	err = server.provider.UpdateConfig(&params)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusBadRequest, errorResp{err.Error()})
		return
	}

	server.writeResp(w, http.StatusOK, &errorResp{})
}

func (server *localServer) withdraw(w http.ResponseWriter, r *http.Request) {
	var payload metaserver.ProviderPaypalWithdrawReq
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusInternalServerError, &errorResp{Error: err.Error()})
		return
	}

	err = server.provider.Withdraw(
		payload.Email,
		payload.Amount,
	)
	if err != nil {
		server.logger.Println(err)
		server.writeResp(w, http.StatusInternalServerError, &errorResp{Error: err.Error()})
		return
	}

	server.writeResp(w, http.StatusOK, &errorResp{})
}

type getTransactionsResp struct {
	Transactions []core.Transaction `json:"transactions"`
}

func (server *localServer) getTransactions(w http.ResponseWriter, r *http.Request) {
	transactions, err := server.provider.ListTransactions()
	if err != nil {
		server.writeResp(w, http.StatusInternalServerError,
			&errorResp{Error: err.Error()})
		return
	}
	server.writeResp(w, http.StatusOK, &getTransactionsResp{transactions})
}

func (server *localServer) writeResp(w http.ResponseWriter, status int, body interface{}) {
	w.WriteHeader(status)
	data, err := json.MarshalIndent(body, "", "    ")
	if err != nil {
		server.logger.Fatalf("error: cannot encode response. error: %s", err)
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
