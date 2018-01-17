package authorization

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"log"
	"net/http"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	jwt "github.com/dgrijalva/jwt-go"
)

type Handshake struct {
	nonce      string
	providerID string
}

// Outstanding handshakes.
var handshakes map[string]Handshake

func InitAuth() {
	handshakes = make(map[string]Handshake)
}

func GetAuthMiddleware(signingKey []byte) *jwtmiddleware.JWTMiddleware {
	return jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return signingKey, nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})
}

type GetAuthChallengeResp struct {
	Nonce string `json:"nonce"`
}

func GetAuthChallengeHandler(userIDString string, logger *log.Logger) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerIDs, present := r.URL.Query()[userIDString]
		if !present {
			w.WriteHeader(http.StatusBadRequest)
			resp := AuthChallengeError{Message: "missing query value: providerID"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if len(providerIDs) < 1 {
			w.WriteHeader(http.StatusBadRequest)
			resp := AuthChallengeError{Message: "must specify a provider"}
			json.NewEncoder(w).Encode(resp)
			return
		} else if len(providerIDs) > 1 {
			w.WriteHeader(http.StatusBadRequest)
			resp := AuthChallengeError{Message: "must specify only 1 provider"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		providerID := providerIDs[0]

		// Generate a nonce signed by the provider's public key
		nonce := randString(8)

		// Record the outstanding handshake
		handshake := Handshake{providerID: providerID, nonce: nonce}
		handshakes[providerID] = handshake

		// Return the nonce to the requester
		resp := GetAuthChallengeResp{Nonce: nonce}
		json.NewEncoder(w).Encode(resp)
	})
}

type AuthChallengeError struct {
	Message string `json:"message"`
}

func GetRespondAuthChallengeHandler(userIDString string, logger *log.Logger, signingKey []byte, getUserPublicKey func(string) (string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.FormValue(userIDString)
		signedNonce := r.FormValue("signedNonce")

		// Make sure the user provided the "providerID" and "signedNonce" arguments
		if userID == "" || signedNonce == "" {
			w.WriteHeader(http.StatusBadRequest)
			resp := AuthChallengeError{Message: "must specify providerID and signedNonce"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Make sure there is an outstanding handshake with the given provider ID
		if _, ok := handshakes[userID]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			resp := AuthChallengeError{Message: "no outstanding handshake with the specified provider"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Retrieve the user's public key.
		publicKeyString, err := getUserPublicKey(userID)
		if err != nil {
			logger.Println(err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		block, _ := pem.Decode([]byte(publicKeyString))
		if block == nil {
			logger.Println("Could not decode PEM.")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			logger.Println("Could not parse public key for provider.")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Convert the Nonce from base64 to bytes
		decoded, err := base64.URLEncoding.DecodeString(signedNonce)
		if err != nil {
			logger.Println("Could not decode signed nonce.")
			w.WriteHeader(http.StatusUnauthorized)
			resp := AuthChallengeError{Message: "could not decode signed nonce"}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Verify the Nonce
		hashed := sha256.Sum256([]byte(handshakes[userID].nonce))

		err = rsa.VerifyPKCS1v15(publicKey.(*rsa.PublicKey), crypto.SHA256, hashed[:], decoded)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			resp := AuthChallengeError{Message: "key verification failed"}
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
		}
	}
}
