package authorization

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	jwt "github.com/dgrijalva/jwt-go"
)

type Handshake struct {
	nonce  string
	userID string
}

type Authorizer struct {
	handshakes map[string]Handshake
	mutex      sync.Mutex
	logger     *log.Logger
}

func NewAuthorizer(logger *log.Logger) Authorizer {
	var authorizer Authorizer
	authorizer.handshakes = make(map[string]Handshake)
	authorizer.logger = logger
	return authorizer
}

type GetAuthChallengeResp struct {
	Nonce string `json:"nonce"`
}

type AuthChallengeError struct {
	Error string `json:"error"`
}

func (authorizer *Authorizer) GetAuthChallengeHandler(userIDString string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDs, present := r.URL.Query()[userIDString]
		if !present {
			w.WriteHeader(http.StatusBadRequest)
			errMsg := fmt.Sprintf("missing query value: %s", userIDString)
			resp := AuthChallengeError{Error: errMsg}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if len(userIDs) != 1 {
			w.WriteHeader(http.StatusBadRequest)
			errMsg := fmt.Sprintf("must specify 1 user with %s", userIDString)
			resp := AuthChallengeError{Error: errMsg}
			json.NewEncoder(w).Encode(resp)
			return
		}

		userID := userIDs[0]

		// Generate a nonce signed by the user's public key
		shaSum := sha256.Sum256([]byte(randString(32)))
		nonce := base64.URLEncoding.EncodeToString(shaSum[:])

		// Record the outstanding handshake
		authorizer.mutex.Lock()
		handshake := Handshake{userID: userID, nonce: nonce}
		authorizer.handshakes[userID] = handshake
		authorizer.mutex.Unlock()

		// Return the nonce to the requester
		resp := GetAuthChallengeResp{Nonce: nonce}
		json.NewEncoder(w).Encode(resp)
	})
}

func (authorizer *Authorizer) GetRespondAuthChallengeHandler(userIDString string, signingKey []byte, getUserPublicKey func(string) (string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.FormValue(userIDString)
		signedNonce := r.FormValue("signedNonce")

		// Make sure the user provided the user ID and "signedNonce" arguments
		if userID == "" || signedNonce == "" {
			w.WriteHeader(http.StatusBadRequest)
			errMsg := fmt.Sprintf("must specify %s and signedNonce", userIDString)
			resp := AuthChallengeError{Error: errMsg}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Make sure there is an outstanding handshake with the given user ID
		var handshake Handshake
		if foundHandshake, ok := authorizer.handshakes[userID]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			resp := AuthChallengeError{Error: "no outstanding handshake for user"}
			json.NewEncoder(w).Encode(resp)
			return
		} else {
			handshake = foundHandshake
		}

		// Retrieve the user's public key.
		publicKeyString, err := getUserPublicKey(userID)
		if err != nil {
			authorizer.logger.Println(err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		block, _ := pem.Decode([]byte(publicKeyString))
		if block == nil {
			authorizer.logger.Println("Could not decode PEM.")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			authorizer.logger.Println("Could not parse public key for user.")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Convert the Nonce from base64 to bytes
		decoded, err := base64.URLEncoding.DecodeString(signedNonce)
		if err != nil {
			authorizer.logger.Println("Could not decode signed nonce.")
			w.WriteHeader(http.StatusUnauthorized)
			resp := AuthChallengeError{Error: "could not decode signed nonce"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Verify the Nonce
		decodedNonce, err := base64.URLEncoding.DecodeString(handshake.nonce)
		if err != nil {
			authorizer.logger.Println("Could not decode stored nonce.")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = rsa.VerifyPKCS1v15(publicKey.(*rsa.PublicKey), crypto.SHA256, decodedNonce[:], decoded)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			resp := AuthChallengeError{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
		} else {
			token := jwt.New(jwt.SigningMethodHS256)

			claims := token.Claims.(jwt.MapClaims)
			claims[userIDString] = userID
			claims["exp"] = time.Now().Add(time.Hour * 24).Unix()

			tokenString, err := token.SignedString(signingKey)
			if err != nil {
				panic(err)
			}
			w.Write([]byte(tokenString))

			authorizer.mutex.Lock()
			delete(authorizer.handshakes, userID)
			authorizer.mutex.Unlock()
		}
	}
}

// This should likely be replaced by middleware that validates the token's claims.
func GetAuthMiddleware(signingKey []byte) *jwtmiddleware.JWTMiddleware {
	return jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return signingKey, nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})
}
