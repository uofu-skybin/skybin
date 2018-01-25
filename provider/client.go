package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"skybin/core"
	"io"
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

func (client *Client) GetInfo() (*core.ProviderInfo, error) {
	// TODO: Implement
	return nil, nil
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
		return nil, errors.New("bad status code")
	}
	return respMsg.Contract, nil
}

func (client *Client) PutBlock(blockID string, renterID string, data io.Reader) error {
	url := fmt.Sprintf("http://%s/blocks/%s", client.addr, blockID)
	resp, err := client.client.Post(url, "application/octet-stream", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var respMsg postContractResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	if resp.StatusCode != http.StatusCreated {
		return errors.New("bad status code")
	}
	return nil
}

func (client *Client) GetBlock(blockID string) (io.ReadCloser, error) {
	url := fmt.Sprintf("http://%s/blocks/%s", client.addr, blockID)
	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, errors.New("bad status code")
	}
	return resp.Body, nil
}

func (client *Client) RemoveBlock(blockID string) error {
	url := fmt.Sprintf("http://%s/blocks/%s", client.addr, blockID)
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
		return errors.New("bad status code")
	}
	return nil
}
