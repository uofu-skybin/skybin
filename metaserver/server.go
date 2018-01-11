package metaserver

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"skybin/core"
	"strconv"

	"github.com/gorilla/mux"
)

func NewServer(homedir string, logger *log.Logger) http.Handler {

	router := mux.NewRouter()

	server := &metaServer{
		homedir:    homedir,
		dbpath:     path.Join(homedir, "metaDB.json"),
		router:     router,
		logger:     logger,
		handshakes: make(map[string]handshake),
	}

	// If the database exists, load it into memory.
	if _, err := os.Stat(server.dbpath); !os.IsNotExist(err) {
		server.loadDbFromFile()
	}

	router.HandleFunc("/auth", server.getAuthChallenge).Methods("GET")
	router.HandleFunc("/auth", server.respondAuthChallenge).Methods("POST")

	router.HandleFunc("/providers", server.getProviders).Methods("GET")
	router.HandleFunc("/providers", server.postProvider).Methods("POST")
	router.HandleFunc("/providers/{id}", server.getProvider).Methods("GET")

	router.HandleFunc("/renters", server.postRenter).Methods("POST")
	router.HandleFunc("/renters/{id}", server.getRenter).Methods("GET")
	router.HandleFunc("/renters/{id}/files", server.getRenterFiles).Methods("GET")
	router.HandleFunc("/renters/{id}/files", server.postRenterFile).Methods("POST")
	router.HandleFunc("/renters/{id}/files/{fileId}", server.getRenterFile).Methods("GET")
	router.HandleFunc("/renters/{id}/files/{fileId}", server.deleteRenterFile).Methods("DELETE")

	return server
}

type handshake struct {
	nonce      string
	providerID string
}

type metaServer struct {
	homedir    string
	dbpath     string
	providers  []core.Provider
	renters    []core.Renter
	logger     *log.Logger
	router     *mux.Router
	handshakes map[string]handshake
}

func (server *metaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.logger.Println(r.Method, r.URL)
	server.router.ServeHTTP(w, r)
}

type storageFile struct {
	Providers []core.Provider
	Renters   []core.Renter
}

func (server *metaServer) dumpDbToFile(providers []core.Provider, renters []core.Renter) {
	db := storageFile{Providers: providers, Renters: renters}

	dbBytes, err := json.Marshal(db)
	if err != nil {
		panic(err)
	}

	writeErr := ioutil.WriteFile(path.Join(server.homedir, server.dbpath), dbBytes, 0644)
	if writeErr != nil {
		panic(err)
	}
}

func (server *metaServer) loadDbFromFile() {

	contents, err := ioutil.ReadFile(path.Join(server.homedir, server.dbpath))
	if err != nil {
		panic(err)
	}

	var db storageFile
	parseErr := json.Unmarshal(contents, &db)
	if parseErr != nil {
		panic(parseErr)
	}

	server.providers = db.Providers
	server.renters = db.Renters
}

type getAuthChallengeResp struct {
	Nonce string `json:"nonce"`
}

func (server *metaServer) getAuthChallenge(w http.ResponseWriter, r *http.Request) {
	providerID := r.URL.Query()["providerID"][0]

	if _, ok := server.handshakes[providerID]; ok {
		w.WriteHeader(http.StatusBadRequest)
		server.logger.Println("Already an outstanding handshake with this provider.")
		return
	}

	// Generate a nonce signed by the provider's public key
	nonce := randString(8)

	// Record the outstanding handshake
	handshake := handshake{providerID: providerID, nonce: nonce}
	server.handshakes[providerID] = handshake

	// Return the nonce to the requester
	resp := getAuthChallengeResp{Nonce: nonce}
	json.NewEncoder(w).Encode(resp)
}

func (server *metaServer) respondAuthChallenge(w http.ResponseWriter, r *http.Request) {
	providerID := r.FormValue("providerID")
	signedNonce := r.FormValue("signedNonce")

	// Make sure the user provided the "providerID" and "signedNonce" arguments
	if providerID == "" || signedNonce == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Make sure there is an outstanding handshake with the given provider ID
	if _, ok := server.handshakes[providerID]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Retrieve the user's public key.
	var provider core.Provider
	foundProvider := false
	for _, item := range server.providers {
		if item.ID == providerID {
			provider = item
			foundProvider = true
		}
	}
	if !foundProvider {
		w.WriteHeader(http.StatusBadRequest)
		server.logger.Println("Could not locate provider.")
		return
	}

	block, _ := pem.Decode([]byte(provider.PublicKey))
	if block == nil {
		panic("Could not decode PEM.")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}

	// Convert the Nonce from base64 to bytes
	decoded, err := base64.StdEncoding.DecodeString(signedNonce)
	if err != nil {
		panic(err)
	}

	// Verify the Nonce
	hashed := sha256.Sum256([]byte(server.handshakes[providerID].nonce))

	err = rsa.VerifyPKCS1v15(publicKey.(*rsa.PublicKey), crypto.SHA256, hashed[:], decoded)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}
}

type getProvidersResp struct {
	Providers []core.Provider `json:"providers"`
	Error     string          `json:"error,omitempty"`
}

func (server *metaServer) getProviders(w http.ResponseWriter, r *http.Request) {
	resp := getProvidersResp{
		Providers: server.providers,
	}
	json.NewEncoder(w).Encode(resp)
}

type postProviderResp struct {
	Provider core.Provider `json:"provider"`
	Error    string        `json:"error,omitempty"`
}

func (server *metaServer) postProvider(w http.ResponseWriter, r *http.Request) {
	var provider core.Provider
	_ = json.NewDecoder(r.Body).Decode(&provider)
	provider.ID = strconv.Itoa(len(server.providers) + 1)
	server.providers = append(server.providers, provider)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(provider)
	server.dumpDbToFile(server.providers, server.renters)
}

func (server *metaServer) getProvider(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.providers {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (server *metaServer) postRenter(w http.ResponseWriter, r *http.Request) {
	var renter core.Renter
	_ = json.NewDecoder(r.Body).Decode(&renter)
	renter.ID = strconv.Itoa(len(server.renters) + 1)
	server.renters = append(server.renters, renter)
	json.NewEncoder(w).Encode(renter)
	server.dumpDbToFile(server.providers, server.renters)
}

func (server *metaServer) getRenter(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (server *metaServer) getRenterFiles(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item.Files)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (server *metaServer) postRenterFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for i, item := range server.renters {
		if item.ID == params["id"] {
			var file core.File
			_ = json.NewDecoder(r.Body).Decode(&file)
			server.renters[i].Files = append(item.Files, file)
			json.NewEncoder(w).Encode(item.Files)
			server.dumpDbToFile(server.providers, server.renters)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (server *metaServer) getRenterFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.renters {
		if item.ID == params["id"] {
			for _, file := range item.Files {
				if file.ID == params["fileId"] {
					json.NewEncoder(w).Encode(file)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (server *metaServer) deleteRenterFile(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range server.renters {
		if item.ID == params["id"] {
			for i, file := range item.Files {
				if file.ID == params["fileId"] {
					item.Files = append(item.Files[:i], item.Files[i+1:]...)
					json.NewEncoder(w).Encode(file)
					server.dumpDbToFile(server.providers, server.renters)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
}
