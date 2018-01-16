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
	"time"

	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
)

const mySigningKey = "secret"

var homedir string
var dbpath string
var providers []core.Provider
var renters []core.Renter
var logger *log.Logger
var router *mux.Router
var handshakes map[string]handshake

func InitServer(homedir string, logger *log.Logger) http.Handler {
	router := mux.NewRouter()

	homedir = homedir
	dbpath = path.Join(homedir, "metaDB.json")
	router = router
	logger = logger
	handshakes = make(map[string]handshake)

	// If the database exists, load it into memory.
	if _, err := os.Stat(dbpath); !os.IsNotExist(err) {
		loadDbFromFile()
	}

	router.Handle("/auth", getAuthChallengeHandler).Methods("GET")
	router.Handle("/auth", respondAuthChallengeHandler).Methods("POST")

	router.Handle("/providers", getProvidersHandler).Methods("GET")
	router.Handle("/providers", postProviderHandler).Methods("POST")
	router.Handle("/providers/{id}", jwtMiddleware.Handler(getProviderHandler)).Methods("GET")

	router.Handle("/renters", postRenterHandler).Methods("POST")
	router.Handle("/renters/{id}", jwtMiddleware.Handler(getRenterHandler)).Methods("GET")
	router.Handle("/renters/{id}/files", jwtMiddleware.Handler(getRenterFilesHandler)).Methods("GET")
	router.Handle("/renters/{id}/files", jwtMiddleware.Handler(postRenterFileHandler)).Methods("POST")
	router.Handle("/renters/{id}/files/{fileId}", jwtMiddleware.Handler(getRenterFileHandler)).Methods("GET")
	router.Handle("/renters/{id}/files/{fileId}", jwtMiddleware.Handler(deleteRenterFileHandler)).Methods("DELETE")

	return router
}

type handshake struct {
	nonce      string
	providerID string
}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Println(r.Method, r.URL)
	router.ServeHTTP(w, r)
}

type storageFile struct {
	Providers []core.Provider
	Renters   []core.Renter
}

func dumpDbToFile(providers []core.Provider, renters []core.Renter) {
	db := storageFile{Providers: providers, Renters: renters}

	dbBytes, err := json.Marshal(db)
	if err != nil {
		panic(err)
	}

	writeErr := ioutil.WriteFile(path.Join(homedir, dbpath), dbBytes, 0644)
	if writeErr != nil {
		panic(err)
	}
}

func loadDbFromFile() {

	contents, err := ioutil.ReadFile(path.Join(homedir, dbpath))
	if err != nil {
		panic(err)
	}

	var db storageFile
	parseErr := json.Unmarshal(contents, &db)
	if parseErr != nil {
		panic(parseErr)
	}

	providers = db.Providers
	renters = db.Renters
}

var jwtMiddleware = jwtmiddleware.New(jwtmiddleware.Options{
	ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
		return mySigningKey, nil
	},
	SigningMethod: jwt.SigningMethodHS256,
})

type getAuthChallengeResp struct {
	Nonce string `json:"nonce"`
}

var getAuthChallengeHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	providerID := r.URL.Query()["providerID"][0]

	if _, ok := handshakes[providerID]; ok {
		w.WriteHeader(http.StatusBadRequest)
		logger.Println("Already an outstanding handshake with this provider.")
		return
	}

	// Generate a nonce signed by the provider's public key
	nonce := randString(8)

	// Record the outstanding handshake
	handshake := handshake{providerID: providerID, nonce: nonce}
	handshakes[providerID] = handshake

	// Return the nonce to the requester
	resp := getAuthChallengeResp{Nonce: nonce}
	json.NewEncoder(w).Encode(resp)
})

var respondAuthChallengeHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	providerID := r.FormValue("providerID")
	signedNonce := r.FormValue("signedNonce")

	// Make sure the user provided the "providerID" and "signedNonce" arguments
	if providerID == "" || signedNonce == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Make sure there is an outstanding handshake with the given provider ID
	if _, ok := handshakes[providerID]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Retrieve the user's public key.
	var provider core.Provider
	foundProvider := false
	for _, item := range providers {
		if item.ID == providerID {
			provider = item
			foundProvider = true
		}
	}
	if !foundProvider {
		w.WriteHeader(http.StatusBadRequest)
		logger.Println("Could not locate provider.")
		return
	}

	block, _ := pem.Decode([]byte(provider.PublicKey))
	if block == nil {
		logger.Fatal("Could not decode PEM.")
		w.WriteHeader(http.StatusUnauthorized)
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		logger.Fatal("Could not parse public key for provider.")
		w.WriteHeader(http.StatusUnauthorized)
	}

	// Convert the Nonce from base64 to bytes
	decoded, err := base64.URLEncoding.DecodeString(signedNonce)
	if err != nil {
		logger.Fatal("Could not decode signed nonce.")
		w.WriteHeader(http.StatusUnauthorized)
	}

	// Verify the Nonce
	hashed := sha256.Sum256([]byte(handshakes[providerID].nonce))

	err = rsa.VerifyPKCS1v15(publicKey.(*rsa.PublicKey), crypto.SHA256, hashed[:], decoded)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		token := jwt.New(jwt.SigningMethodHS256)

		claims := token.Claims.(jwt.MapClaims)
		claims["providerID"] = provider.ID
		claims["exp"] = time.Now().Add(time.Hour * 24).Unix()

		tokenString, _ := token.SignedString(mySigningKey)
		w.Write([]byte(tokenString))
	}
})

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

var postRenterHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	var renter core.Renter
	_ = json.NewDecoder(r.Body).Decode(&renter)
	renter.ID = strconv.Itoa(len(renters) + 1)
	renters = append(renters, renter)
	json.NewEncoder(w).Encode(renter)
	dumpDbToFile(providers, renters)
})

var getRenterHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
})

var getRenterFilesHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item.Files)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
})

var postRenterFileHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for i, item := range renters {
		if item.ID == params["id"] {
			var file core.File
			_ = json.NewDecoder(r.Body).Decode(&file)
			renters[i].Files = append(item.Files, file)
			json.NewEncoder(w).Encode(item.Files)
			dumpDbToFile(providers, renters)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
})

var getRenterFileHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
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
})

var deleteRenterFileHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range renters {
		if item.ID == params["id"] {
			for i, file := range item.Files {
				if file.ID == params["fileId"] {
					item.Files = append(item.Files[:i], item.Files[i+1:]...)
					json.NewEncoder(w).Encode(file)
					dumpDbToFile(providers, renters)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
})
