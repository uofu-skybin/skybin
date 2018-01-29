package renter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

func (client *Client) GetInfo() (*Info, error) {
	url := fmt.Sprintf("http://%s/info", client.addr)

	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var info Info
	err = json.NewDecoder(resp.Body).Decode(&info)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

func (client *Client) ReserveStorage(amount int64) ([]*core.Contract, error) {
	url := fmt.Sprintf("http://%s/reserve-storage", client.addr)

	req := reserveStorageReq{
		Amount: amount,
	}
	data, _ := json.Marshal(&req)
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp.Body)
	}

	var respMsg reserveStorageResp
	err = json.NewDecoder(resp.Body).Decode(&respMsg)
	if err != nil {
		return nil, err
	}

	return respMsg.Contracts, nil
}

func (client *Client) Upload(srcPath, destPath string) (*core.File, error) {
	url := fmt.Sprintf("http://%s/files/upload", client.addr)

	req := uploadFileReq{
		SourcePath: srcPath,
		DestPath:   destPath,
	}
	data, _ := json.Marshal(&req)
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp.Body)
	}

	file := &core.File{}
	err = json.NewDecoder(resp.Body).Decode(file)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (client *Client) Download(fileId string, destpath string) error {
	url := fmt.Sprintf("http://%s/files/download", client.addr)
	req := downloadFileReq{
		FileId:   fileId,
		DestPath: destpath,
	}
	data, _ := json.Marshal(&req)
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) CreateFolder(name string) (*core.File, error) {
	url := fmt.Sprintf("http://%s/files/create-folder", client.addr)
	req := createFolderReq{
		Name: name,
	}
	data, _ := json.Marshal(&req)
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp.Body)
	}

	file := &core.File{}
	err = json.NewDecoder(resp.Body).Decode(file)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (client *Client) ListFiles() ([]*core.File, error) {
	url := fmt.Sprintf("http://%s/files", client.addr)

	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

func (client *Client) Remove(fileId string) error {
	url := fmt.Sprintf("http://%s/files/remove", client.addr)

	req := removeFileReq{
		FileID: fileId,
	}
	data, _ := json.Marshal(&req)
	resp, err := client.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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
