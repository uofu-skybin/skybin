package metaserver

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path"
	"runtime"
	"skybin/authorization"
	"skybin/core"

	"github.com/gorilla/mux"
)

func getStaticPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("could not locate static files dir")
	}
	return path.Join(path.Dir(filename), "static"), nil
}

// InitServer prepares a handler for the server.
func InitServer(dataDirectory string, showDash bool, logger *log.Logger) *MetaServer {
	router := mux.NewRouter()

	db, err := newMongoDB()
	if err != nil {
		panic(err)
	}

	server := &MetaServer{
		dataDir:    dataDirectory,
		db:         db,
		router:     router,
		logger:     logger,
		authorizer: authorization.NewAuthorizer(logger),
		signingKey: []byte("secret"),
	}

	authMiddleware := authorization.GetAuthMiddleware(server.signingKey)

	router.Handle("/auth/provider", server.authorizer.GetAuthChallengeHandler("providerID")).Methods("GET")
	router.Handle("/auth/provider", server.authorizer.GetRespondAuthChallengeHandler(
		"providerID", server.signingKey, server.getProviderPublicKey)).Methods("POST")

	router.Handle("/auth/renter", server.authorizer.GetAuthChallengeHandler("renterID")).Methods("GET")
	router.Handle("/auth/renter", server.authorizer.GetRespondAuthChallengeHandler(
		"renterID", server.signingKey, server.getRenterPublicKey)).Methods("POST")

	router.Handle("/providers", server.getProvidersHandler()).Methods("GET")
	router.Handle("/providers", server.postProviderHandler()).Methods("POST")
	router.Handle("/providers/{id}", server.getProviderHandler()).Methods("GET")
	router.Handle("/providers/{id}", authMiddleware.Handler(server.putProviderHandler())).Methods("PUT")
	router.Handle("/providers/{id}", authMiddleware.Handler(server.deleteProviderHandler())).Methods("DELETE")

	router.Handle("/renters", server.postRenterHandler()).Methods("POST")
	router.Handle("/renters", server.getRenterByAliasHandler()).Queries("alias", "{alias}").Methods("GET")
	router.Handle("/renters/{id}", authMiddleware.Handler(server.getRenterHandler())).Methods("GET")
	router.Handle("/renters/{id}", authMiddleware.Handler(server.putRenterHandler())).Methods("PUT")
	router.Handle("/renters/{id}", authMiddleware.Handler(server.deleteRenterHandler())).Methods("DELETE")

	router.Handle("/renters/{renterID}/contracts", authMiddleware.Handler(server.getContractsHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/contracts", authMiddleware.Handler(server.postContractHandler())).Methods("POST")
	router.Handle("/renters/{renterID}/contracts/{contractID}", authMiddleware.Handler(server.getContractHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/contracts/{contractID}", authMiddleware.Handler(server.putContractHandler())).Methods("PUT")
	router.Handle("/renters/{renterID}/contracts/{contractID}", authMiddleware.Handler(server.deleteContractHandler())).Methods("DELETE")
	router.Handle("/renters/{renterID}/contracts/{contractID}/payment", authMiddleware.Handler(server.getContractPaymentHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/contracts/{contractID}/payment", authMiddleware.Handler(server.putContractPaymentHandler())).Methods("PUT")

	router.Handle("/renters/{renterID}/files", authMiddleware.Handler(server.getFilesHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files", authMiddleware.Handler(server.postFileHandler())).Methods("POST")
	router.Handle("/renters/{renterID}/files/{fileID}", authMiddleware.Handler(server.getFileHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}", authMiddleware.Handler(server.putFileHandler())).Methods("PUT")
	router.Handle("/renters/{renterID}/files/{fileID}", authMiddleware.Handler(server.deleteFileHandler())).Methods("DELETE")
	router.Handle("/renters/{renterID}/files/{fileID}/versions", authMiddleware.Handler(server.getFileVersionsHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}/versions", authMiddleware.Handler(server.postFileVersionHandler())).Methods("POST")
	router.Handle("/renters/{renterID}/files/{fileID}/versions/{version}", authMiddleware.Handler(server.getFileVersionHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}/versions/{version}", authMiddleware.Handler(server.putFileVersionHandler())).Methods("PUT")
	router.Handle("/renters/{renterID}/files/{fileID}/versions/{version}", authMiddleware.Handler(server.deleteFileVersionHandler())).Methods("DELETE")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions", authMiddleware.Handler(server.getFilePermissionsHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions", authMiddleware.Handler(server.postFilePermissionHandler())).Methods("POST")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions/{sharedID}", authMiddleware.Handler(server.getFilePermissionHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions/{sharedID}", authMiddleware.Handler(server.putFilePermissionHandler())).Methods("PUT")
	router.Handle("/renters/{renterID}/files/{fileID}/permissions/{sharedID}", authMiddleware.Handler(server.deleteFilePermissionHandler())).Methods("DELETE")

	router.Handle("/renters/{renterID}/shared", authMiddleware.Handler(server.getSharedFilesHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/shared/{fileID}", authMiddleware.Handler(server.getFileHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/shared/{fileID}", authMiddleware.Handler(server.deleteSharedFileHandler())).Methods("DELETE")
	router.Handle("/renters/{renterID}/shared/{fileID}/versions", authMiddleware.Handler(server.getFileVersionsHandler())).Methods("GET")
	router.Handle("/renters/{renterID}/shared/{fileID}/versions/{version}", authMiddleware.Handler(server.getFileVersionHandler())).Methods("GET")

	router.Handle("/paypal/create", authMiddleware.Handler(server.getCreatePaypalPaymentHandler())).Methods("POST")
	router.Handle("/paypal/execute", authMiddleware.Handler(server.getExecutePaypalPaymentHandler())).Methods("POST")
	router.Handle("/paypal/renter-withdraw", authMiddleware.Handler(server.getRenterPaypalWithdrawHandler())).Methods("POST")
	router.Handle("/paypal/provider-withdraw", authMiddleware.Handler(server.getProviderPaypalWithdrawHandler())).Methods("POST")

	if showDash {
		router.Handle("/dashboard.json", server.getDashboardDataHandler()).Methods("GET")

		staticPath, err := getStaticPath()
		if err != nil {
			server.logger.Fatal(err)
		}
		router.Handle("/{someFile}", http.FileServer(http.Dir(staticPath)))
	}

	server.startPaymentRunner()

	return server
}

type MetaServer struct {
	dataDir    string
	db         *mongoDB
	providers  []core.ProviderInfo
	renters    []core.RenterInfo
	logger     *log.Logger
	router     *mux.Router
	authorizer authorization.Authorizer
	signingKey []byte
}

type errorResp struct {
	Error string `json:"error"`
}

func writeErr(msg string, status int, w http.ResponseWriter) {
	w.WriteHeader(status)
	resp := errorResp{Error: msg}
	json.NewEncoder(w).Encode(resp)
	return
}

func writeAndLogInternalError(err interface{}, w http.ResponseWriter, l *log.Logger) {
	w.WriteHeader(http.StatusInternalServerError)
	l.Println(err)
	resp := errorResp{Error: "internal server error"}
	json.NewEncoder(w).Encode(resp)
	return
}

// ServeHTTP begins serving requests from the server's router.
func (server *MetaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
	server.router.ServeHTTP(w, r)
}

func (server *MetaServer) Close() {
	server.db.CloseDB()
}

type dashboardResp struct {
	Providers []core.ProviderInfo `json:"providers"`
	Renters   []core.RenterInfo   `json:"renters"`
	Contracts []core.Contract     `json:"contracts"`
	Files     []core.File         `json:"files"`
}

func (server *MetaServer) getDashboardDataHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providers, err := server.db.FindAllProviders()
		if err != nil {
			writeAndLogInternalError(err.Error(), w, server.logger)
			return
		}
		renters, err := server.db.FindAllRenters()
		if err != nil {
			writeAndLogInternalError(err.Error(), w, server.logger)
			return
		}
		contracts, err := server.db.FindAllContracts()
		if err != nil {
			writeAndLogInternalError(err.Error(), w, server.logger)
			return
		}
		files, err := server.db.FindAllFiles()
		if err != nil {
			writeAndLogInternalError(err.Error(), w, server.logger)
			return
		}

		resp := dashboardResp{
			Providers: providers,
			Renters:   renters,
			Contracts: contracts,
			Files:     files,
		}

		json.NewEncoder(w).Encode(resp)
	})
}

func (server *MetaServer) getDashboardHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			server.logger.Println("could not get package directory")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.ServeFile(w, r, path.Join(path.Dir(filename), "public"))
	})
}
