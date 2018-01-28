package metaserver

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"skybin/core"
	"testing"

	"github.com/go-test/deep"
)

func registerRenter(client *Client, alias string) (*core.RenterInfo, error) {
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

	err = client.RegisterRenter(&renter)
	if err != nil {
		return nil, err
	}

	renterID := fingerprintKey(publicKeyString)
	renter.ID = renterID

	err = client.Authorize(rsaKey, renterID)
	if err != nil {
		return nil, errors.New("could not authorize")
	}

	return &renter, nil
}

// Check that renters can register with the metaserver.
func TestMetaserverRenterRegistration(t *testing.T) {
	// Create a client for testing.
	httpClient := http.Client{}
	client := NewClient(core.DefaultMetaAddr, &httpClient)

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
		Alias:     "testRenterReg",
	}

	err = client.RegisterRenter(&renter)
	if err != nil {
		t.Fatal("encountered error while registering renter: ", err)
	}

	renterID := fingerprintKey(publicKeyString)

	err = client.Authorize(rsaKey, renterID)
	if err != nil {
		t.Fatal("encountered error while attempting to authorize with registered renter: ", err)
	}

	expected := &core.RenterInfo{
		PublicKey: publicKeyString,
		Alias:     "testRenterReg",
		ID:        renterID,
	}

	returned, err := client.GetRenter(renterID)
	if err != nil {
		t.Fatal("encountered error while attempting to retrieve registered renter info: ", err)
	}

	if returned.Alias != expected.Alias || returned.ID != expected.ID || returned.PublicKey != expected.PublicKey {
		t.Fatal("Expected ", expected, ", received ", expected)
	}
}

// Update renter
func TestUpdateRenter(t *testing.T) {
	httpClient := http.Client{}
	client := NewClient(core.DefaultMetaAddr, &httpClient)

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

// Get renter info
// Delete renter

// POST contract
// Get contracts
// Get contract
// Delete contract

// Register provider
// Update provider
// List providers
// Get provider
// Delete provider

// Get files
// Get file
// Get shared files
// Get shared file
// POST file
// Update file
// Get file
// Delete file
// Post new version of file
// Get all versions of file
// Get version of file
// Delete version of file
// Share file
// Remove shared file
