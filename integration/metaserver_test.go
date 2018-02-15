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

func TestGetRenterByAlias(t *testing.T) {
	// Create a client for testing.
	httpClient := http.Client{}
	client := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter.
	_, err := registerRenter(client, "getRenterByAliasTest")
	if err != nil {
		t.Fatal(err)
	}

	otherRenter, err := registerRenter(otherClient, "getRenterByAliasTest2")
	if err != nil {
		t.Fatal(err)
	}

	expected := &core.RenterInfo{
		PublicKey: otherRenter.PublicKey,
		Alias:     otherRenter.Alias,
		ID:        otherRenter.ID,
	}

	returned, err := client.GetRenterByAlias(otherRenter.Alias)
	if err != nil {
		t.Fatal("encountered error while attempting to retrieve renter by alias: ", err)
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
	sharerClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	shareeClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	sharer, err := registerRenter(sharerClient, "fileShareSharerTest")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter
	sharedWith, err := registerRenter(shareeClient, "fileShareSharedWithTest")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(sharerClient, sharer.ID, "testShareFile", "testShareFile")
	if err != nil {
		t.Fatal(err)
	}

	// Share the file
	permission := core.Permission{
		RenterId: sharedWith.ID,
	}
	err = sharerClient.ShareFile(sharer.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file shows up in the sharee's files.
	_, err = shareeClient.GetSharedFile(sharedWith.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file shows up in the file's ACL.
	result, err := sharerClient.GetFile(sharer.ID, file.ID)
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
	sharerClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	shareeClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	sharer, err := registerRenter(sharerClient, "fileGetSharedFilesSharerTest")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter
	sharedWith, err := registerRenter(shareeClient, "fileGetSharedFilesSharedWithTest")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(sharerClient, sharer.ID, "testGetSharedFiles", "testGetSharedFiles")
	if err != nil {
		t.Fatal(err)
	}

	// Share the file
	permission := core.Permission{
		RenterId: sharedWith.ID,
	}
	err = sharerClient.ShareFile(sharer.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file shows up in the sharee's files
	files, err := shareeClient.GetSharedFiles(sharedWith.ID)
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
	sharerClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	shareeClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	sharer, err := registerRenter(sharerClient, "fileUnshareSharerTest")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter
	sharedWith, err := registerRenter(shareeClient, "fileUnshareSharedWithTest")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(sharerClient, sharer.ID, "testUnshareFile", "testUnshareFile")
	if err != nil {
		t.Fatal(err)
	}

	// Share the file
	permission := core.Permission{
		RenterId: sharedWith.ID,
	}
	err = sharerClient.ShareFile(sharer.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Unshare the file.
	err = sharerClient.UnshareFile(sharer.ID, file.ID, sharedWith.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file doesn't show up in the sharee' directory.
	files, err := shareeClient.GetSharedFiles(sharedWith.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range files {
		if item.ID == file.ID {
			t.Fatal("shared file remained in user's directory")
		}
	}

	// Make sure the file's ACL is empty
	result, err := sharerClient.GetFile(sharer.ID, file.ID)
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
	sharerClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	shareeClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	sharer, err := registerRenter(sharerClient, "fileRemoveSharedSharerTest")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter
	sharedWith, err := registerRenter(shareeClient, "fileRemoveSharedSharedWithTest")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(sharerClient, sharer.ID, "testRemoveSharedFile", "testRemoveSharedFile")
	if err != nil {
		t.Fatal(err)
	}

	// Share the file
	permission := core.Permission{
		RenterId: sharedWith.ID,
	}
	err = sharerClient.ShareFile(sharer.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Remove the shared file from sharee's directory
	err = shareeClient.RemoveSharedFile(sharedWith.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the file doesn't show up in the user's directory.
	files, err := shareeClient.GetSharedFiles(sharedWith.ID)
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

func TestRenterGetFilesAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testGetFilesAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	_, err = registerRenter(otherRenterClient, "testGetFilesAuth2")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the first renter's files with the second renter.
	_, err = otherRenterClient.GetFiles(renter.ID)
	if err == nil {
		t.Fatal("no error when accessing other renter's files")
	}
}

func TestRenterPostFileAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testPostFileAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	_, err = registerRenter(otherRenterClient, "testPostFileAuth2")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to post a file to the first renter's directory with the second renter.
	file := core.File{
		ID:   "renterPostFileAuthTest",
		Name: "renterPostFileAuthTest",
	}
	err = otherRenterClient.PostFile(renter.ID, &file)
	if err == nil {
		t.Fatal("no error when posting to other renter's directory")
	}
}

func TestRenterGetFileAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testGetFileAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testGetFileAuth2")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testGetFileAuth", "testGetFileAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the file with the other renter.
	_, err = otherRenterClient.GetFile(otherRenter.ID, file.ID)
	if err == nil {
		t.Fatal("no error when accessing unshared file")
	}

	// Share the file
	permission := core.Permission{
		RenterId: otherRenter.ID,
	}
	err = renterClient.ShareFile(renter.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the file with the (now) authorized renter
	_, err = otherRenterClient.GetFile(otherRenter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenterDeleteFileAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterDeleteFileAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testRenterDeleteFileAuth2")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testDeleteFileAuth", "testDeleteFileAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to delete the uploaded file with the other renter.
	err = otherRenterClient.DeleteFile(otherRenter.ID, file.ID)
	if err == nil {
		t.Fatal("no error when deleting other user's file")
	}

	// Make sure the file still exists
	_, err = renterClient.GetFile(renter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

}

func TestRenterPutFileAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterPutFileAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testRenterPutFileAuth2")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testPutFileAuth", "testPutFileAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to delete the uploaded file with the other renter.
	newFile := &core.File{
		ID:      file.ID,
		Name:    "foo",
		OwnerID: file.OwnerID,
	}
	err = otherRenterClient.UpdateFile(otherRenter.ID, newFile)
	if err == nil {
		t.Fatal("no error when modifying other user's file")
	}

	// Make sure the file has not been modified
	resultFile, err := renterClient.GetFile(renter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(file, resultFile); diff != nil {
		t.Fatal(diff)
	}

}

func TestRenterGetFileVersionAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testGetFileVersionAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testGetFileVersionAuth2")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testGetFileVersionAuth", "testGetFileVersionAuth")
	if err != nil {
		t.Fatal(err)
	}

	version := &core.Version{}
	err = renterClient.PostFileVersion(renter.ID, file.ID, version)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the file with the other renter.
	_, err = otherRenterClient.GetFileVersion(otherRenter.ID, file.ID, 1)
	if err == nil {
		t.Fatal("no error when accessing unshared file version")
	}

	// Share the file
	permission := core.Permission{
		RenterId: otherRenter.ID,
	}
	err = renterClient.ShareFile(renter.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the file with the (now) authorized renter
	_, err = otherRenterClient.GetFileVersion(otherRenter.ID, file.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenterGetFileVersionsAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testGetFileVersionsAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testGetFileVersionsAuth2")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testGetFileVersionsAuth", "testGetFileVersionsAuth")
	if err != nil {
		t.Fatal(err)
	}

	version := &core.Version{}
	err = renterClient.PostFileVersion(renter.ID, file.ID, version)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the file with the other renter.
	_, err = otherRenterClient.GetFileVersions(otherRenter.ID, file.ID)
	if err == nil {
		t.Fatal("no error when accessing unshared file versions")
	}

	// Share the file
	permission := core.Permission{
		RenterId: otherRenter.ID,
	}
	err = renterClient.ShareFile(renter.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the file with the (now) authorized renter
	_, err = otherRenterClient.GetFileVersions(otherRenter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenterDeleteVersionAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterDeleteversionAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	_, err = registerRenter(otherRenterClient, "testRenterDeleteversionAuth2")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testDeleteversionAuth", "testDeleteversionAuth")
	if err != nil {
		t.Fatal(err)
	}

	version := &core.Version{}
	err = renterClient.PostFileVersion(renter.ID, file.ID, version)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to delete the uploaded version with the other renter.
	err = otherRenterClient.DeleteFileVersion(renter.ID, file.ID, 1)
	if err == nil {
		t.Fatal("no error when deleting other user's version")
	}

	// Make sure the file still exists
	_, err = renterClient.GetFile(renter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

}

func TestRenterPostVersionAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testPostVersionAuth")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testPostVersionAuth", "testPostVersionAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testPostVersionAuth2")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to post a version to the first renter's file with the second renter.
	version := &core.Version{}
	err = otherRenterClient.PostFileVersion(otherRenter.ID, file.ID, version)
	if err == nil {
		t.Fatal("no error when posting version to other renter's file")
	}
}

func TestRenterPutVersionAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterPutVersionAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testRenterPutVersionAuth2")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testPutVersionAuth", "testPutVersionAuth")
	if err != nil {
		t.Fatal(err)
	}

	version := &core.Version{Num: 1, Blocks: make([]core.Block, 0)}
	err = renterClient.PostFileVersion(renter.ID, file.ID, version)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to modify the version with the other renter.
	newVersion := &core.Version{
		Num:           1,
		NumDataBlocks: 1,
	}

	err = otherRenterClient.PutFileVersion(otherRenter.ID, file.ID, newVersion)
	if err == nil {
		t.Fatal("no error when putting other user's file version")
	}

	// Make sure the file has not been modified
	resultVersion, err := renterClient.GetFileVersion(renter.ID, file.ID, 1)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(version, resultVersion); diff != nil {
		t.Fatal(diff)
	}

}

func TestRenterDeletePermissionAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	lastRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterDeletePermissionAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	_, err = registerRenter(otherRenterClient, "testRenterDeletePermissionAuth2")
	if err != nil {
		t.Fatal(err)
	}

	lastRenter, err := registerRenter(lastRenterClient, "testRenterDeletePermissionAuth3")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testDeletePermissionAuth", "testDeletePermissionAuth")
	if err != nil {
		t.Fatal(err)
	}

	permission := &core.Permission{RenterId: lastRenter.ID}
	err = renterClient.ShareFile(renter.ID, file.ID, permission)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to delete the uploaded permission with the other renter.
	err = otherRenterClient.UnshareFile(renter.ID, file.ID, lastRenter.ID)
	if err == nil {
		t.Fatal("no error when deleting other user's permission")
	}

	// Make sure the other file is still accessible by the user
	_, err = lastRenterClient.GetFile(renter.ID, file.ID)
	if err != nil {
		t.Fatal(err)
	}

}

func TestRenterPostPermissionAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testPostPermissionAuth")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testPostPermissionAuth", "testPostPermissionAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testPostPermissionAuth2")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to post a version to the first renter's file with the second renter.
	permission := &core.Permission{}
	err = otherRenterClient.ShareFile(otherRenter.ID, file.ID, permission)
	if err == nil {
		t.Fatal("no error when posting permission to other renter's file")
	}
}

func TestRenterGetContractsAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testGetContractsAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	_, err = registerRenter(otherRenterClient, "testGetContractsAuth2")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the first renter's files with the second renter.
	_, err = otherRenterClient.GetRenterContracts(renter.ID)
	if err == nil {
		t.Fatal("no error when accessing other renter's contracts")
	}
}

func TestRenterPostContractAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testPostContractAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	_, err = registerRenter(otherRenterClient, "testPostContractAuth2")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to post a contract to the first renter's file with the second renter.
	contract := &core.Contract{RenterId: renter.ID}
	err = otherRenterClient.PostContract(renter.ID, contract)
	if err == nil {
		t.Fatal("no error when posting contract for other renter")
	}
}

func TestRenterGetContractAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testGetContractAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testGetContractAuth2")
	if err != nil {
		t.Fatal(err)
	}

	contract := &core.Contract{ID: "testGetContractAuth", RenterId: renter.ID}
	err = renterClient.PostContract(renter.ID, contract)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the file with the other renter.
	_, err = otherRenterClient.GetContract(otherRenter.ID, contract.ID)
	if err == nil {
		t.Fatal("no error when accessing other renters contract")
	}
}

func TestRenterDeleteContractAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterDeleteContractAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testRenterDeleteContractAuth2")
	if err != nil {
		t.Fatal(err)
	}

	contract := &core.Contract{ID: "testDeleteContractAuth", RenterId: renter.ID}
	err = renterClient.PostContract(renter.ID, contract)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to delete the uploaded file with the other renter.
	err = otherRenterClient.DeleteContract(otherRenter.ID, contract.ID)
	if err == nil {
		t.Fatal("no error when deleting other user's contract")
	}

	// Make sure the file still exists
	_, err = renterClient.GetContract(renter.ID, contract.ID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenterGetSharedFilesAuth(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testGetSharedFilesAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	_, err = registerRenter(otherRenterClient, "testGetSharedFilesAuth2")
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to access the first renter's files with the second renter.
	_, err = otherRenterClient.GetSharedFiles(renter.ID)
	if err == nil {
		t.Fatal("no error when accessing other renter's shared files")
	}
}

func TestRenterDeleteSharedFileAuthentication(t *testing.T) {
	httpClient := http.Client{}
	renterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	otherRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)
	lastRenterClient := metaserver.NewClient(core.DefaultMetaAddr, &httpClient)

	// Register a renter
	renter, err := registerRenter(renterClient, "testRenterDeleteSharedFileAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Register another renter.
	otherRenter, err := registerRenter(otherRenterClient, "testRenterDeleteSharedFileAuth2")
	if err != nil {
		t.Fatal(err)
	}

	_, err = registerRenter(lastRenterClient, "testRenterDeleteSharedFileAuth3")
	if err != nil {
		t.Fatal(err)
	}

	file, err := uploadFile(renterClient, renter.ID, "testDeleteSharedFileAuth", "testDeleteSharedFileAuth")
	if err != nil {
		t.Fatal(err)
	}

	// Share the file
	permission := core.Permission{
		RenterId: otherRenter.ID,
	}
	err = renterClient.ShareFile(renter.ID, file.ID, &permission)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to remove the shared file with the last renter.
	err = lastRenterClient.RemoveSharedFile(otherRenter.ID, file.ID)
	if err == nil {
		t.Fatal("no error when deleting other user's file")
	}
}
