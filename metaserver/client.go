package metaserver

import (
	"net/http"
	"skybin/core"
	"encoding/json"
	"fmt"
	"errors"
	"bytes"
)

func NewClient(addr string, client *http.Client) *Client {
	return &Client{
		addr: addr,
		client: client,
	}
}

type Client struct {
	addr string
	client *http.Client
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
