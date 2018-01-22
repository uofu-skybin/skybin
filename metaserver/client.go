package metaserver

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"skybin/authorization"
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
	token  string
}

func (client *Client) Authorize(privateKey *rsa.PrivateKey, renterID string) error {
	authClient := authorization.NewClient(client.addr, client.client)
	token, err := authClient.GetAuthToken(privateKey, "renter", renterID)
	if err != nil {
		return err
	}
	client.token = token
	return nil
}

func (client *Client) RegisterProvider(info *core.ProviderInfo) error {
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

func (client *Client) GetProviders() ([]core.ProviderInfo, error) {
	url := fmt.Sprintf("http://%s/providers", client.addr)

	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}

	var respMsg getProvidersResp
	err = json.NewDecoder(resp.Body).Decode(&respMsg)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(respMsg.Error)
	}

	return respMsg.Providers, nil
}

func (client *Client) GetProvider(providerID string) (core.ProviderInfo, error) {
	url := fmt.Sprintf("http://%s/providers/%s", client.addr, providerID)

	resp, err := client.client.Get(url)
	if err != nil {
		return core.ProviderInfo{}, err
	}

	var respMsg core.ProviderInfo
	err = json.NewDecoder(resp.Body).Decode(&respMsg)
	if err != nil {
		return core.ProviderInfo{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return core.ProviderInfo{}, errors.New("bad status from server")
	}

	return respMsg, nil
}

func (client *Client) RegisterRenter(info *core.RenterInfo) error {
	url := fmt.Sprintf("http://%s/renters", client.addr)
	body, err := json.Marshal(info)
	if err != nil {
		return err
	}
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		var respMsg postRenterResp
		err = json.NewDecoder(resp.Body).Decode(&respMsg)
		if err != nil {
			return err
		}
		return errors.New(respMsg.Error)
	}
	return nil
}

func (client *Client) GetRenter(renterID string) (core.RenterInfo, error) {
	if client.token == "" {
		return core.RenterInfo{}, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s", client.addr, renterID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return core.RenterInfo{}, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if resp.StatusCode != http.StatusCreated {
		var respMsg postRenterResp
		err = json.NewDecoder(resp.Body).Decode(&respMsg)
		if err != nil {
			return core.RenterInfo{}, err
		}
		return core.RenterInfo{}, errors.New(respMsg.Error)
	}

	var renter core.RenterInfo
	err = json.NewDecoder(resp.Body).Decode(&renter)
	if err != nil {
		return core.RenterInfo{}, err
	}
	return renter, nil
}

func (client *Client) PostFile(file core.File) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/files", client.addr)

	var buf []byte
	serialized := bytes.NewBuffer(buf)
	_ = json.NewEncoder(serialized).Encode(file)
	body := bytes.NewReader(buf)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return errors.New("bad status from server")
	}

	return nil
}

func (client *Client) GetFile(fileID string) (core.File, error) {
	if client.token == "" {
		return core.File{}, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/files/%s", client.addr, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return core.File{}, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return core.File{}, err
	}

	var file core.File
	err = json.NewDecoder(resp.Body).Decode(&file)
	if err != nil {
		return core.File{}, err
	}

	return file, nil
}

func (client *Client) GetFileVersion(fileID string, fileVersion int) (core.File, error) {
	if client.token == "" {
		return core.File{}, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/files/%s/%d", client.addr, fileID, fileVersion)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return core.File{}, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return core.File{}, err
	}

	var file core.File
	err = json.NewDecoder(resp.Body).Decode(&file)
	if err != nil {
		return core.File{}, err
	}

	return file, nil
}

func (client *Client) DeleteFile(fileID string) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/files/%s", client.addr, fileID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("Bad response from server")
	}

	return nil
}

func (client *Client) DeleteFileVersion(fileID string, fileVersion int) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/files/%s/%d", client.addr, fileID, fileVersion)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("Bad response from server")
	}

	return nil
}
