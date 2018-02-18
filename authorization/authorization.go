package authorization

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"crypto/rand"

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

// Generates a challenge string to be signed by the user's private key.
func makeNonce() (string, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(buf)
	return base64.URLEncoding.EncodeToString(h[:]), nil
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

		// Generate a nonce to be signed by the user's private key
		nonce, err := makeNonce()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			resp := AuthChallengeError{Error: "Unable to generate authorization challenge."}
			json.NewEncoder(w).Encode(&resp)
			return
		}

		// Record the outstanding handshake
		authorizer.mutex.Lock()
		// if the user already has a nonce associated with the id use that
		if _, present := authorizer.handshakes[userID]; present {
			nonce = authorizer.handshakes[userID].nonce
		}
		handshake := Handshake{userID: userID, nonce: nonce}
		authorizer.handshakes[userID] = handshake
		authorizer.mutex.Unlock()

		// Return the nonce to the requester
		resp := GetAuthChallengeResp{Nonce: nonce}
		json.NewEncoder(w).Encode(resp)
	})
}

func (authorizer *Authorizer) GetRespondAuthChallengeHandler(userIDString string, signingKey []byte,
	getUserPublicKey func(string) (*rsa.PublicKey, error)) http.HandlerFunc {
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
		handshake, exists := authorizer.handshakes[userID]
		if !exists {
			w.WriteHeader(http.StatusBadRequest)
			resp := AuthChallengeError{Error: "no outstanding handshake for user"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Retrieve the user's public key.
		publicKey, err := getUserPublicKey(userID)
		if err != nil {
			authorizer.logger.Println(err)
			w.WriteHeader(http.StatusUnauthorized)
			resp := AuthChallengeError{Error: "could not find user's public key"}
			json.NewEncoder(w).Encode(resp)
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
			resp := AuthChallengeError{Error: "internal server error"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, decodedNonce[:], decoded)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			resp := AuthChallengeError{Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
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
		// delete(authorizer.handshakes, userID)
		authorizer.mutex.Unlock()
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
