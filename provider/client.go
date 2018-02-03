package provider

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"skybin/authorization"
	"skybin/core"
)

type Client struct {
	client *http.Client
	addr   string
	token  string
}

func NewClient(addr string, client *http.Client) *Client {
	return &Client{
		client: client,
		addr:   addr,
	}
}

func (client *Client) AuthorizeRenter(privateKey *rsa.PrivateKey, renterID string) error {
	authClient := authorization.NewClient(client.addr, client.client)
	token, err := authClient.GetAuthToken(privateKey, "renter", renterID)
	if err != nil {
		return err
	}
	client.token = token
	return nil
}

func (client *Client) ReserveStorage(contract *core.Contract) (*core.Contract, error) {
	url := fmt.Sprintf("http://%s/contracts", client.addr)
	body, err := json.Marshal(&postContractParams{Contract: contract})
	if err != nil {
		return nil, err
	}
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respMsg postContractResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp.Body)
	}
	return respMsg.Contract, nil
}

func (client *Client) RenewContract(contract *core.Contract) (*core.Contract, error) {
	url := fmt.Sprintf("http://%s/contracts/renew", client.addr)
	body, err := json.Marshal(&postContractParams{Contract: contract})
	if err != nil {
		return nil, err
	}
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respMsg postContractResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp.Body)
	}
	return respMsg.Contract, nil
}

func (client *Client) PutBlock(renterID string, blockID string, data io.Reader) error {
	url := fmt.Sprintf("http://%s/blocks?renterID=%s&blockID=%s", client.addr, renterID, blockID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Post(url, "application/octet-stream", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return decodeError(resp.Body)
	}
	return nil
}

func (client *Client) GetBlock(renterID string, blockID string) (io.ReadCloser, error) {
	url := fmt.Sprintf("http://%s/blocks?renterID=%s&blockID=%s", client.addr, renterID, blockID)
	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, decodeError(resp.Body)
	}
	return resp.Body, nil
}

func (client *Client) AuditBlock(renterID string, blockID string) (io.ReadCloser, error) {
	url := fmt.Sprintf("http://%s/blocks/audit?renterID=%s&blockID=%s", client.addr, renterID, blockID)
	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}
	return resp.Body, nil
}

func (client *Client) RemoveBlock(renterID string, blockID string) error {
	url := fmt.Sprintf("http://%s/blocks?renterID=%s&blockID=%s", client.addr, renterID, blockID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeError(resp.Body)
	}
	return nil
}

func (client *Client) GetInfo() (*core.ProviderInfo, error) {
	url := fmt.Sprintf("http://%s/info", client.addr)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	provInfo := &core.ProviderInfo{}
	err = json.NewDecoder(resp.Body).Decode(provInfo)
	if err != nil {
		return nil, err
	}

	return provInfo, nil

}

func (client *Client) GetRenterInfo() (*RenterInfo, error) {
	url := fmt.Sprintf("http://%s/renter-info", client.addr)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}
	renterInfo := &RenterInfo{}
	err = json.NewDecoder(resp.Body).Decode(renterInfo)
	if err != nil {
		return nil, err
	}
	return renterInfo, nil
}

func decodeError(r io.Reader) error {
	var respMsg errorResp
	err := json.NewDecoder(r).Decode(&respMsg)
	if err != nil {
		return err
	}
	return errors.New(respMsg.Error)
}
