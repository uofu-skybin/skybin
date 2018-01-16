package metaserver

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"skybin/core"
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

func (client *Client) Authorize(privateKey *rsa.PrivateKey) (string, error) {
	challengeURL := fmt.Sprintf("http://%s/auth?providerID=1", client.addr)

	// Get a challenge token
	resp, err := client.client.Get(challengeURL)
	if err != nil {
		return "", err
	}
	var respMsg getAuthChallengeResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	token := respMsg.Nonce

	// Sign the token
	hashed := sha256.Sum256([]byte(token))

	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}

	// Encode the token and send it back to the server.
	encoded := base64.URLEncoding.EncodeToString(signature)

	respondURL := fmt.Sprintf("http://%s/auth", client.addr)
	resp, err = client.client.PostForm(respondURL, url.Values{"providerID": {"1"}, "signedNonce": {encoded}})
	if err != nil {
		return "", err
	} else {
		println(resp.StatusCode)
		var b []byte
		defer resp.Body.Close()
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		if resp.StatusCode != 200 {
			panic("Bad status: " + resp.Status)
		}
		return string(b), nil
	}
}

func (client *Client) GetProviders() ([]core.Provider, error) {
	url := fmt.Sprintf("http://%s/providers", client.addr)
	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}
	var respMsg getProvidersResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(respMsg.Error)
	}
	return respMsg.Providers, nil
}

func (client *Client) RegisterProvider(info *core.Provider) error {
	url := fmt.Sprintf("http://%s/providers", client.addr)
	body, err := json.Marshal(info)
	if err != nil {
		return err
	}
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		var respMsg postProviderResp
		err = json.NewDecoder(resp.Body).Decode(&respMsg)
		if err != nil {
			return err
		}
		return errors.New(respMsg.Error)
	}
	return nil
}
