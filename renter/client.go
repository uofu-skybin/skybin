package renter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"skybin/core"
	"io"
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

func (client *Client) ReserveStorage(amount int64) ([]*core.Contract, error) {
	url := fmt.Sprintf("http://%s/storage", client.addr)

	req := postStorageReq{Amount: amount}
	data, _ := json.Marshal(&req)
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp.Body)
	}

	var respMsg postStorageResp
	err = json.NewDecoder(resp.Body).Decode(&respMsg)
	if err != nil {
		return nil, err
	}

	return respMsg.Contracts, nil
}

func (client *Client) Upload(srcPath, destPath string) (*core.File, error) {
	url := fmt.Sprintf("http://%s/files", client.addr)

	req := postFilesReq{
		SourcePath: srcPath,
		DestPath:   destPath,
	}
	data, _ := json.Marshal(&req)
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp.Body)
	}

	var respMsg postFilesResp
	err = json.NewDecoder(resp.Body).Decode(&respMsg)
	if err != nil {
		return nil, err
	}

	return respMsg.File, nil
}

func (client *Client) CreateFolder(name string) (*core.File, error) {
	return client.Upload("", name)
}

func (client *Client) ListFiles() ([]*core.File, error) {
	url := fmt.Sprintf("http://%s/files", client.addr)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var respMsg getFilesResp
	err = json.NewDecoder(resp.Body).Decode(&respMsg)
	if err != nil {
		return nil, err
	}

	return respMsg.Files, nil
}

func (client *Client) Download(fileId string, destpath string) error {
	url := fmt.Sprintf("http://%s/files/%s/download", client.addr, fileId)

	req := postDownloadReq{Destination: destpath}
	data, _ := json.Marshal(&req)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return decodeError(resp.Body)
	}

	return nil
}

func decodeError(r io.Reader) error {
	var respMsg errorResp
	err := json.NewDecoder(r).Decode(&respMsg)
	if err != nil {
		return err
	}
	return errors.New(respMsg.Error)
}
