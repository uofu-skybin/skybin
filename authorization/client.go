package authorization

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

func NewClient(addr string, client *http.Client) *Client {
	return &Client{
		addr:   addr,
		client: client,
	}
}

type Client struct {
	addr   string
	client *http.Client
}

func (client *Client) GetAuthToken(privateKey *rsa.PrivateKey, authType string, userID string) (string, error) {
	challengeURL := fmt.Sprintf("http://%[1]s/auth/%[2]s?%[2]sID=%[3]s", client.addr, authType, userID)

	// Get a challenge token
	resp, err := client.client.Get(challengeURL)
	if err != nil {
		return "", err
	}
	var respMsg GetAuthChallengeResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	token := respMsg.Nonce

	// Sign the token
	nonce, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}

	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, nonce)
	if err != nil {
		return "", err
	}

	// Encode the token and send it back to the server.
	encoded := base64.URLEncoding.EncodeToString(signature)

	respondURL := fmt.Sprintf("http://%[1]s/auth/%[2]s", client.addr, authType)
	responseField := fmt.Sprintf("%sID", authType)
	resp, err = client.client.PostForm(respondURL, url.Values{responseField: {userID}, "signedNonce": {encoded}})
	if err != nil {
		return "", err
	} else {
		var b []byte
		defer resp.Body.Close()
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		if resp.StatusCode != 200 {
			return "", errors.New(string(b))
		}
		return string(b), nil
	}
}
