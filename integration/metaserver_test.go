package metaserver

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"skybin/core"
	"skybin/metaserver"
	"testing"
	"time"

	"github.com/go-test/deep"
)

func fingerprintKey(key string) string {
	shaSum := sha256.Sum256([]byte(key))
	fingerprint := hex.EncodeToString(shaSum[:])
	return fingerprint
}

func getPublicKeyString(key *rsa.PublicKey) string {
	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		panic(err)
	}
	keyBlock := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: keyBytes,
	}
	buf := bytes.NewBuffer(make([]byte, 0))
	err = pem.Encode(buf, keyBlock)
	if err != nil {
		panic("could not encode PEM block")
	}
	return buf.String()
}

func registerRenter(client *metaserver.Client, alias string) (*core.RenterInfo, error) {
	// Generate an RSA key for registration.
	reader := rand.Reader
	rsaKey, err := rsa.GenerateKey(reader, 2048)
	publicKeyString := getPublicKeyString(&rsaKey.PublicKey)
	if err != nil {
		panic("could not generate rsa key")
	}

	// Renter that will be registered.
	renter := core.RenterInfo{
		PublicKey: publicKeyString,
		Alias:     alias,
		Shared:    make([]string, 0),
		Files:     make([]string, 0),
	}

	_, err = client.RegisterRenter(&renter)
	if err != nil {
		return nil, err
	}

	renterID := fingerprintKey(publicKeyString)
	renter.ID = renterID

	err = client.AuthorizeRenter(rsaKey, renterID)
	if err != nil {
		return nil, errors.New("could not authorize")
	}

	return &renter, nil
}

func registerProvider(client *metaserver.Client) (*core.ProviderInfo, error) {
	// Generate an RSA key for registration.
	reader := rand.Reader
	rsaKey, err := rsa.GenerateKey(reader, 2048)
	publicKeyString := getPublicKeyString(&rsaKey.PublicKey)
	if err != nil {
		panic("could not generate rsa key")
	}

	// provider that will be registered.
	provider := core.ProviderInfo{
		PublicKey:   publicKeyString,
		Addr:        "foo",
		SpaceAvail:  500,
		StorageRate: 5,
	}

	_, err = client.RegisterProvider(&provider)
	if err != nil {
		return nil, err
	}

	provider.ID = fingerprintKey(publicKeyString)

	err = client.AuthorizeProvider(rsaKey, provider.ID)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

// Check that renters can register with the metaserver.
func TestRegisterRenter(t *testing.T) {
	// Create a client for testing.
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter.
	renter, err := registerRenter(client, "renterRegisterTest")
	if err != nil {
		t.Fatal(err)
	}

	expected := &core.RenterInfo{
		PublicKey: renter.PublicKey,
		Alias:     "renterRegisterTest",
		ID:        renter.ID,
		Shared:    make([]string, 0),
		Files:     make([]string, 0),
	}

	returned, err := client.GetRenter(renter.ID)
	if err != nil {
		t.Fatal("encountered error while attempting to retrieve registered renter info: ", err)
	}

	if diff := deep.Equal(expected, returned); diff != nil {
		t.Fatal(diff)
	}
}

// Update renter
func TestUpdateRenter(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter.
	renter, err := registerRenter(client, "renterUpdateTest")
	if err != nil {
		t.Fatal(err)
	}

	// Update the renter's information.
	renter.Files = append(renter.Files, "hi")
	err = client.UpdateRenter(renter)
	if err != nil {
		t.Fatal(err)
	}

	updatedRenter, err := client.GetRenter(renter.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(renter, updatedRenter); diff != nil {
		t.Fatal(diff)
	}

	// Make sure we can't update the ID, alias, or public key.
	oldID := renter.ID
	renter.ID = "foo"
	err = client.UpdateRenter(renter)
	if err == nil {
		t.Fatal("was able to update renter ID")
	}
	renter.ID = oldID

	oldAlias := renter.Alias
	renter.Alias = "foo"
	err = client.UpdateRenter(renter)
	if err == nil {
		t.Fatal("was able to update renter alias")
	}
	renter.Alias = oldAlias

	renter.PublicKey = "foo"
	err = client.UpdateRenter(renter)
	if err == nil {
		t.Fatal("was able to update renter public key")
	}
}

// Delete renter
func TestDeleteRenter(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter.
	renter, err := registerRenter(client, "renterDeleteTest")
	if err != nil {
		t.Fatal(err)
	}

	// Delete the renter.
	err = client.DeleteRenter(renter.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to get the renter's information.
	_, err = client.GetRenter(renter.ID)
	if err == nil {
		t.Fatal("was able to retrieve deleted renter")
	}
}

// POST contract
func TestPostContract(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(client, "contractUploadTest")
	if err != nil {
		t.Fatal(err)
	}

	contract := core.Contract{
		ID:       "contractUploadTest",
		RenterId: renter.ID,
	}
	err = client.PostContract(renter.ID, &contract)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetContract(renter.ID, contract.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(contract, *result); diff != nil {
		t.Fatal(diff)
	}
}

// Get contracts
func TestGetContracts(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(client, "contractGetTest")
	if err != nil {
		t.Fatal(err)
	}

	// POST a few contracts
	var contracts []core.Contract
	for i := 0; i < 10; i++ {
		contractName := fmt.Sprintf("contractGetTest%d", i)
		contract := core.Contract{
			ID:       contractName,
			RenterId: renter.ID,
		}
		err = client.PostContract(renter.ID, &contract)
		if err != nil {
			t.Fatal(err)
		}
		contracts = append(contracts, contract)
	}

	result, err := client.GetRenterContracts(renter.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, contract := range contracts {
		compared := false
		for _, item := range result {
			if item.ID == contract.ID {
				if diff := deep.Equal(contract, item); diff != nil {
					t.Fatal(diff)
				}
				compared = true
				break
			}
		}
		if !compared {
			t.Fatal("Contract ", contract.ID, " missing from output")
		}
	}
}

// Delete contract
func TestDeleteContract(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter.
	renter, err := registerRenter(client, "contractDeleteTest")
	if err != nil {
		t.Fatal(err)
	}

	contract := core.Contract{
		ID:       "contractDeleteTest",
		RenterId: renter.ID,
	}
	err = client.PostContract(renter.ID, &contract)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to delete the file
	err = client.DeleteContract(renter.ID, contract.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to retrieve the file.
	_, err = client.GetContract(renter.ID, contract.ID)
	if err == nil {
		t.Fatal("was able to retrieve deleted contract")
	}
}

// Register provider
func TestRegisterProvider(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	provider, err := registerProvider(client)
	if err != nil {
		t.Fatal(err)
	}

	returned, err := client.GetProvider(provider.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(provider, returned); diff != nil {
		t.Fatal(diff)
	}
}

// Update provider
func TestUpdateProvider(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	provider, err := registerProvider(client)
	if err != nil {
		t.Fatal(err)
	}

	provider.Addr = "thisisanew.address"
	err = client.UpdateProvider(provider)
	if err != nil {
		t.Fatal(err)
	}

	updatedProvider, err := client.GetProvider(provider.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(provider, updatedProvider); diff != nil {
		t.Fatal(diff)
	}

	// Make sure we can't update public key or id
	oldID := provider.ID
	provider.ID = "foo"
	err = client.UpdateProvider(provider)
	if err == nil {
		t.Fatal("was able to update provider ID")
	}
	provider.ID = oldID

	provider.PublicKey = "foo"
	err = client.UpdateProvider(provider)
	if err == nil {
		t.Fatal("was able to update provider public key")
	}
}

// Delete provider
func TestDeleteProvider(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a provider.
	provider, err := registerProvider(client)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the provider.
	err = client.DeleteProvider(provider.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to get the provider's information.
	_, err = client.GetProvider(provider.ID)
	if err == nil {
		t.Fatal("was able to retrieve deleted provider")
	}
}

func uploadFile(client *metaserver.Client, renterID string, ID string, name string) (*core.File, error) {
	// Attempt to upload a file.
	file := core.File{
		ID:         ID,
		Name:       name,
		AccessList: make([]core.Permission, 0),
		Versions:   make([]core.Version, 0),
	}
	err := client.PostFile(renterID, &file)
	if err != nil {
		return nil, err
	}
	file.OwnerID = renterID
	return &file, nil
}

// POST file
func TestUploadFile(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(client, "fileUploadTest")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(client, renter.ID, "testUploadFile", "testUploadFile")
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve the file and make sure it matches what we posted and has the proper owner.
	result, err := client.GetFile(renter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	file.OwnerID = renter.ID
	if diff := deep.Equal(file, result); diff != nil {
		t.Fatal(diff)
	}

	// Make sure the file shows up in the renter's files.
	renter, err = client.GetRenter(renter.ID)
	if err != nil {
		t.Fatal(err)
	}

	foundFile := false
	for _, item := range renter.Files {
		if item == file.ID {
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Fatal("file not added to renter's directory")
	}
}

// Update file
func TestUpdateFile(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(client, "fileUpdateTest")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to upload a file.
	file, err := uploadFile(client, renter.ID, "fileUpdateTest", "fileUpdateTest")
	if err != nil {
		t.Fatal(err)
	}

	// Update the file.
	file.Name = "newName"
	err = client.UpdateFile(renter.ID, file)
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve the file and make sure it matches what we posted and has the proper owner.
	result, err := client.GetFile(renter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	file.OwnerID = renter.ID
	if diff := deep.Equal(file, result); diff != nil {
		t.Fatal(diff)
	}

	// Make sure the file shows up in the renter's files.
	renter, err = client.GetRenter(renter.ID)
	if err != nil {
		t.Fatal(err)
	}

	foundFile := false
	for _, item := range renter.Files {
		if item == file.ID {
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Fatal("file not added to renter's directory")
	}

	// Make sure users can't update file's owner ID
	file.OwnerID = "somebodyElseShouldPay"
	err = client.UpdateFile(renter.ID, file)
	if err == nil {
		t.Fatal("user allowed to update owner ID")
	}
}

// Get renter's files
func TestGetFiles(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(client, "filesGetTest")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to upload some files.
	var files []*core.File
	for i := 1; i < 10; i++ {
		fileName := fmt.Sprintf("getFilesTest%d", i)
		file, err := uploadFile(client, renter.ID, fileName, fileName)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}

	// Retrieve the files and make sure they are all present.
	result, err := client.GetFiles(renter.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != len(files) {
		t.Fatal("wrong number of files returned")
	}

	for _, file := range files {
		compared := false
		for _, item := range result {
			if item.ID == file.ID {
				if diff := deep.Equal(*file, item); diff != nil {
					t.Fatal(diff)
				}
				compared = true
				break
			}
		}
		if !compared {
			t.Fatal("File ", file.ID, " missing from output")
		}
	}
}

// Delete file
func TestDeleteFile(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter.
	renter, err := registerRenter(client, "fileDeleteTest")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to upload a file.
	file, err := uploadFile(client, renter.ID, "fileDeleteTest", "fileDeleteTest")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to delete the file
	err = client.DeleteFile(renter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to retrieve the file.
	_, err = client.GetFile(renter.ID, file.ID)
	if err == nil {
		t.Fatal("was able to retrieve deleted file")
	}
}

// Post new version of file
func TestUploadNewFileVersion(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(client, "fileUploadVersionTest")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to upload a file.
	file, err := uploadFile(client, renter.ID, "testUploadFileVersion", "testUploadFileVersion")
	if err != nil {
		t.Fatal(err)
	}

	// Upload a new version of the file.
	version := core.Version{
		Blocks:  make([]core.Block, 0),
		Size:    1000,
		ModTime: time.Time{},
	}

	err = client.PostFileVersion(renter.ID, file.ID, &version)
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve the version and make sure it matches.
	version.Num = 1

	result, err := client.GetFileVersion(renter.ID, file.ID, 1)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(version, *result); diff != nil {
		t.Fatal(diff)
	}
}

// Get all versions of file
func TestGetFileVersions(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(client, "fileVersionsGetTest")
	if err != nil {
		t.Fatal(err)
	}

	// Upload a file.
	file, err := uploadFile(client, renter.ID, "getFileVersionsTest", "getFileVersionsTest")
	if err != nil {
		t.Fatal(err)
	}

	// upload some versions.
	var versions []core.Version
	for i := 0; i < 10; i++ {
		version := core.Version{
			Blocks:  make([]core.Block, 0),
			Size:    1000,
			ModTime: time.Time{},
		}
		err = client.PostFileVersion(renter.ID, file.ID, &version)
		if err != nil {
			t.Fatal(err)
		}
		version.Num = i + 1
		versions = append(versions, version)
	}

	// Retrieve the versions and make sure they are all present.
	result, err := client.GetFileVersions(renter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, version := range versions {
		compared := false
		for _, item := range result {
			if item.Num == version.Num {
				if diff := deep.Equal(version, item); diff != nil {
					t.Fatal(diff)
				}
				compared = true
				break
			}
		}
		if !compared {
			t.Fatal("Version ", version.Num, " missing from output")
		}
	}
}

// Delete version of file
func TestDeleteFileVersion(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(client, "fileDeleteVersionTest")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to upload a file.
	file, err := uploadFile(client, renter.ID, "testDeleteFileVersion", "testDeleteFileVersion")
	if err != nil {
		t.Fatal(err)
	}

	// Upload a new version of the file.
	version := core.Version{
		Blocks:  make([]core.Block, 0),
		Size:    1000,
		ModTime: time.Time{},
	}

	err = client.PostFileVersion(renter.ID, file.ID, &version)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the version of the file.
	err = client.DeleteFileVersion(renter.ID, file.ID, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure we can't retrieve that version.
	result, err := client.GetFileVersion(renter.ID, file.ID, 1)
	if err == nil {
		t.Log(result)
		t.Fatal("was able to retrieve deleted file version")
	}
}

// Update file version
func TestUpdateFileVersion(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(client, "fileUpdateVersionTest")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to upload a file.
	file, err := uploadFile(client, renter.ID, "testUpdateFileVersion", "testUpdateFileVersion")
	if err != nil {
		t.Fatal(err)
	}

	// Upload a new version of the file.
	version := core.Version{
		Blocks:  make([]core.Block, 0),
		Size:    1000,
		ModTime: time.Time{},
	}

	err = client.PostFileVersion(renter.ID, file.ID, &version)
	if err != nil {
		t.Fatal(err)
	}

	// Update the version we just uploaded.
	version.Num = 1
	version.Size = 2000

	err = client.PutFileVersion(renter.ID, file.ID, &version)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file was updated.
	result, err := client.GetFileVersion(renter.ID, file.ID, 1)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(version, *result); diff != nil {
		t.Fatal(diff)
	}
}

// Share file
func TestShareFile(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	sharer, err := registerRenter(client, "fileShareSharerTest")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter
	sharedWith, err := registerRenter(client, "fileShareSharedWithTest")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(client, sharer.ID, "testShareFile", "testShareFile")
	if err != nil {
		t.Fatal(err)
	}

	// Share the file
	permission := core.Permission{
		RenterId: sharedWith.ID,
	}
	err = client.ShareFile(sharer.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file shows up in the sharee's files.
	_, err = client.GetSharedFile(sharedWith.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file shows up in the file's ACL.
	result, err := client.GetFile(sharer.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	foundPermission := false
	for _, item := range result.AccessList {
		if item.RenterId == sharedWith.ID {
			foundPermission = true
		}
	}

	if !foundPermission {
		t.Fatal("file not added to renter's directory")
	}
}

// Get shared files
func TestGetSharedFiles(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	sharer, err := registerRenter(client, "fileGetSharedFilesSharerTest")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter
	sharedWith, err := registerRenter(client, "fileGetSharedFilesSharedWithTest")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(client, sharer.ID, "testGetSharedFiles", "testGetSharedFiles")
	if err != nil {
		t.Fatal(err)
	}

	// Share the file
	permission := core.Permission{
		RenterId: sharedWith.ID,
	}
	err = client.ShareFile(sharer.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file shows up in the sharee's files
	files, err := client.GetSharedFiles(sharedWith.ID)
	if err != nil {
		t.Fatal(err)
	}

	foundFile := false
	for _, item := range files {
		if item.ID == file.ID {
			foundFile = true
			break
		}
	}

	if !foundFile {
		t.Fatal("could not locate shared file")
	}
}

// Unshare file
func TestUnshareFile(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	sharer, err := registerRenter(client, "fileUnshareSharerTest")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter
	sharedWith, err := registerRenter(client, "fileUnshareSharedWithTest")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(client, sharer.ID, "testUnshareFile", "testUnshareFile")
	if err != nil {
		t.Fatal(err)
	}

	// Share the file
	permission := core.Permission{
		RenterId: sharedWith.ID,
	}
	err = client.ShareFile(sharer.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Unshare the file.
	err = client.UnshareFile(sharer.ID, file.ID, sharedWith.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the fil doesn't show up in the sharee' directory.
	files, err := client.GetSharedFiles(sharedWith.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range files {
		if item.ID == file.ID {
			t.Fatal("shared file remained in user's directory")
		}
	}

	// Make sure the file's ACL is empty
	result, err := client.GetFile(sharer.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AccessList) > 0 {
		t.Fatal("permission remained in file after unsharing")
	}
}

// Remove shared file
func TestRemoveSharedFile(t *testing.T) {
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	sharer, err := registerRenter(client, "fileRemoveSharedSharerTest")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter
	sharedWith, err := registerRenter(client, "fileRemoveSharedSharedWithTest")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(client, sharer.ID, "testRemoveSharedFile", "testRemoveSharedFile")
	if err != nil {
		t.Fatal(err)
	}

	// Share the file
	permission := core.Permission{
		RenterId: sharedWith.ID,
	}
	err = client.ShareFile(sharer.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Unshare the file.
	err = client.RemoveSharedFile(sharedWith.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file doesn't show up in the user's directory.
	files, err := client.GetSharedFiles(sharedWith.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range files {
		if item.ID == file.ID {
			t.Fatal("shared file remained in user's directory")
		}
	}
}

func TestGetRenterAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	providerClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterGetAuth")
	if err != nil {
		t.Fatal(err)
	}

	// "Upload" a file as the renter
	file := core.File{
		ID:   "renterGetAuthTest",
		Name: "renterGetAuthTest",
	}
	err = renterClient.PostFile(renter.ID, &file)
	if err != nil {
		t.Fatal(err)
	}

	// Register a provider.
	_, err = registerProvider(providerClient)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the renter can retrieve all of its information.
	renterInfo, err := renterClient.GetRenter(renter.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Information should match renter (plus file we uploaded).
	renter.Files = append(renter.Files, file.ID)
	if diff := deep.Equal(renter, renterInfo); diff != nil {
		t.Fatal(diff)
	}

	// Make sure provider cannot access non-public renter info
	renterInfo, err = providerClient.GetRenter(renter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(renterInfo.Files) != 0 {
		t.Fatal("files included with renter")
	}
}

func TestPutRenterAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterPutAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	_, err = registerRenter(otherRenterClient, "testRenterPutAuth2")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to modify the first renter with the second.
	newInfo := &core.RenterInfo{
		ID:    renter.ID,
		Alias: "foo",
	}
	err = otherRenterClient.UpdateRenter(newInfo)
	if err == nil {
		t.Fatal("no error when modifying other user")
	}

	// Make sure the original renter was unaffected
	resultInfo, err := renterClient.GetRenter(renter.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(renter, resultInfo); diff != nil {
		t.Fatal(diff)
	}

}

func TestDeleteRenterAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterDeleteAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	_, err = registerRenter(otherRenterClient, "testRenterDeleteAuth2")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to modify the first renter with the second.
	err = otherRenterClient.DeleteRenter(renter.ID)
	if err == nil {
		t.Fatal("no error when deleting other user")
	}

	// Make sure the original renter was unaffected
	_, err = renterClient.GetRenter(renter.ID)
	if err != nil {
		t.Fatal(err)
	}

}

func TestPutProviderAuthentication(t *testing.T) {
	httpClient := http.Client{}
	providerClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherProviderClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a provider
	provider, err := registerProvider(providerClient)
	if err != nil {
		t.Fatal(err)
	}

	// Register another provider.
	_, err = registerProvider(otherProviderClient)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to modify the first provider with the second.
	newInfo := &core.ProviderInfo{
		ID:   provider.ID,
		Addr: "foo",
	}
	err = otherProviderClient.UpdateProvider(newInfo)
	if err == nil {
		t.Fatal("no error when modifying other user")
	}

	// Make sure the original provider was unaffected
	resultInfo, err := providerClient.GetProvider(provider.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(provider, resultInfo); diff != nil {
		t.Fatal(diff)
	}

}

func TestDeleteProviderAuthentication(t *testing.T) {
	httpClient := http.Client{}
	providerClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherProviderClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a provider
	provider, err := registerProvider(providerClient)
	if err != nil {
		t.Fatal(err)
	}

	// Register another provider.
	_, err = registerProvider(otherProviderClient)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to delete the first provider with the second.
	err = otherProviderClient.DeleteProvider(provider.ID)
	if err == nil {
		t.Fatal("no error when deleting other user")
	}

	// Make sure the original provider was unaffected
	_, err = providerClient.GetProvider(provider.ID)
	if err != nil {
		t.Fatal(err)
	}

}
