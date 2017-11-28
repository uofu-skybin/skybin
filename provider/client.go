package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	var respMsg postContractResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	if resp.StatusCode != http.StatusCreated {
		return nil, errors.New("bad status code")
	}
	return respMsg.Contract, nil
}

func (client *Client) PutBlock(blockID string, data []byte) error {
	url := fmt.Sprintf("http://%s/blocks/%s", client.addr, blockID)
	body, err := json.Marshal(&postBlockParams{Data: data})
	if err != nil {
		return err
	}
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	var respMsg postContractResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	if resp.StatusCode != http.StatusCreated {
		return errors.New("bad status code")
	}
	return nil
}

func (client *Client) GetBlock(blockID string) ([]byte, error) {
	url := fmt.Sprintf("http://%s/blocks/%s", client.addr, blockID)
	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}
	var respMsg getBlockResp
	_ = json.NewDecoder(resp.Body).Decode(&respMsg)
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("bad status code")
	}
	return respMsg.Data, nil
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
	if resp.StatusCode != http.StatusOK {
		return errors.New("bad status code")
	}
	return nil
}
