package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"skybin/core"
)

type Client struct {
	client *http.Client
	addr   string
}

func NewClient(addr string, client *http.Client) *Client {
	return &Client{
		client: client,
		addr:   addr,
	}
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
		return nil, err
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
		return nil, err
	}
	return respMsg.Contract, nil
}

func (client *Client) PutBlock(renterID string, blockID string, data io.Reader) error {
	url := fmt.Sprintf("http://%s/blocks?renterID=%s&blockID=%s", client.addr, renterID, blockID)
	resp, err := client.client.Post(url, "application/octet-stream", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var respMsg postContractResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	if resp.StatusCode != http.StatusCreated {
		return err
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
		resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
}

func (client *Client) RemoveBlock(renterID string, blockID string) error {
	url := fmt.Sprintf("http://%s/blocks?renterID=%s&blockID=%s", client.addr, renterID, blockID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return err
	}

	return nil
}

func (client *Client) GetInfo() (*core.ProviderInfo, error) {
	url := fmt.Sprintf("http://%s/info", client.addr)
	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, err
	}
	var provInfo *core.ProviderInfo
	err = json.NewDecoder(resp.Body).Decode(&provInfo)

	if err != nil {
		return nil, err
	}

	return provInfo, nil

}

func (client *Client) GetRenterInfo() (*RenterInfo, error) {
	url := fmt.Sprintf("http://%s/renter-info", client.addr)
	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, err
	}
	var renterInfo *RenterInfo
	err = json.NewDecoder(resp.Body).Decode(&renterInfo)

	if err != nil {
		return nil, err
	}

	return renterInfo, nil
}
