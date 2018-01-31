package metaserver

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

func decodeError(r io.Reader) error {
	var respMsg errorResp
	err := json.NewDecoder(r).Decode(&respMsg)
	if err != nil {
		return err
	}
	return errors.New(respMsg.Error)
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

func (client *Client) AuthorizeProvider(privateKey *rsa.PrivateKey, providerID string) error {
	authClient := authorization.NewClient(client.addr, client.client)
	token, err := authClient.GetAuthToken(privateKey, "provider", providerID)
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
		return decodeError(resp.Body)
	}
	return nil
}

func (client *Client) GetProviders() ([]core.ProviderInfo, error) {
	url := fmt.Sprintf("http://%s/providers", client.addr)

	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var respMsg getProvidersResp
	err = json.NewDecoder(resp.Body).Decode(&respMsg)
	if err != nil {
		return nil, err
	}

	return respMsg.Providers, nil
}

func (client *Client) GetProvider(providerID string) (*core.ProviderInfo, error) {
	url := fmt.Sprintf("http://%s/providers/%s", client.addr, providerID)

	resp, err := client.client.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var respMsg core.ProviderInfo
	err = json.NewDecoder(resp.Body).Decode(&respMsg)
	if err != nil {
		return nil, err
	}

	return &respMsg, nil
}

func (client *Client) UpdateProvider(provider *core.ProviderInfo) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/providers/%s", client.addr, provider.ID)

	b, err := json.Marshal(provider)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(b))
	if err != nil {
		return err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) DeleteProvider(providerID string) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/providers/%s", client.addr, providerID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp.Body)
	}

	return nil
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
		return decodeError(resp.Body)
	}
	return nil
}

func (client *Client) GetRenter(renterID string) (*core.RenterInfo, error) {
	if client.token == "" {
		return nil, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s", client.addr, renterID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var renter core.RenterInfo
	err = json.NewDecoder(resp.Body).Decode(&renter)
	if err != nil {
		return nil, err
	}
	return &renter, nil
}

func (client *Client) UpdateRenter(renter *core.RenterInfo) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s", client.addr, renter.ID)

	b, err := json.Marshal(renter)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(b))
	if err != nil {
		return err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) DeleteRenter(renterID string) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s", client.addr, renterID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) PostFile(renterID string, file *core.File) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files", client.addr, renterID)

	b, err := json.Marshal(file)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) UpdateFile(renterID string, file *core.File) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s", client.addr, renterID, file.ID)

	b, err := json.Marshal(file)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(b))
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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) GetFile(renterID string, fileID string) (*core.File, error) {
	if client.token == "" {
		return nil, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s", client.addr, renterID, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var file core.File
	err = json.NewDecoder(resp.Body).Decode(&file)
	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (client *Client) GetFiles(renterID string) ([]core.File, error) {
	if client.token == "" {
		return nil, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files", client.addr, renterID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var files []core.File
	err = json.NewDecoder(resp.Body).Decode(&files)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (client *Client) DeleteFile(renterID string, fileID string) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s", client.addr, renterID, fileID)

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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) PostFileVersion(renterID string, fileID string, version *core.Version) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s/versions", client.addr, renterID, fileID)

	b, err := json.Marshal(version)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) PutFileVersion(renterID string, fileID string, version *core.Version) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s/versions/%d", client.addr, renterID, fileID, version.Number)

	b, err := json.Marshal(version)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(b))
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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) GetFileVersion(renterID string, fileID string, fileVersion int) (*core.Version, error) {
	if client.token == "" {
		return nil, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s/versions/%d", client.addr, renterID, fileID, fileVersion)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var version core.Version
	err = json.NewDecoder(resp.Body).Decode(&version)
	if err != nil {
		return nil, err
	}

	return &version, nil
}

func (client *Client) GetFileVersions(renterID string, fileID string) ([]core.Version, error) {
	if client.token == "" {
		return nil, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s/versions", client.addr, renterID, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var versions []core.Version
	err = json.NewDecoder(resp.Body).Decode(&versions)
	if err != nil {
		return nil, err
	}

	return versions, nil
}

func (client *Client) DeleteFileVersion(renterID string, fileID string, fileVersion int) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s/versions/%d", client.addr, renterID, fileID, fileVersion)

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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) GetSharedFile(renterID string, fileID string) (*core.File, error) {
	if client.token == "" {
		return nil, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/shared/%s", client.addr, renterID, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var file core.File
	err = json.NewDecoder(resp.Body).Decode(&file)
	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (client *Client) ShareFile(renterID string, fileID string, permission *core.Permission) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s/permissions", client.addr, renterID, fileID)

	b, err := json.Marshal(permission)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) UnshareFile(renterID string, fileID string, userID string) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/files/%s/permissions/%s", client.addr, renterID, fileID, userID)

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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) GetSharedFiles(renterID string) ([]core.File, error) {
	if client.token == "" {
		return nil, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/shared", client.addr, renterID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var files []core.File
	err = json.NewDecoder(resp.Body).Decode(&files)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (client *Client) RemoveSharedFile(renterID string, fileID string) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/shared/%s", client.addr, renterID, fileID)

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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) PostContract(renterID string, contract *core.Contract) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/contracts", client.addr, renterID)

	b, err := json.Marshal(contract)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
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
		return decodeError(resp.Body)
	}

	return nil
}

func (client *Client) GetContract(renterID string, contractID string) (*core.Contract, error) {
	if client.token == "" {
		return nil, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/contracts/%s", client.addr, renterID, contractID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var contract core.Contract
	err = json.NewDecoder(resp.Body).Decode(&contract)
	if err != nil {
		return nil, err
	}

	return &contract, nil
}

func (client *Client) GetRenterContracts(renterID string) ([]core.Contract, error) {
	if client.token == "" {
		return nil, errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/contracts", client.addr, renterID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("Bearer %s", client.token)
	req.Header.Add("Authorization", token)

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}

	var contracts []core.Contract
	err = json.NewDecoder(resp.Body).Decode(&contracts)
	if err != nil {
		return nil, err
	}

	return contracts, nil
}

func (client *Client) DeleteContract(renterID string, contractID string) error {
	if client.token == "" {
		return errors.New("must authorize before calling this method")
	}

	url := fmt.Sprintf("http://%s/renters/%s/contracts/%s", client.addr, renterID, contractID)

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
		return decodeError(resp.Body)
	}

	return nil
}
