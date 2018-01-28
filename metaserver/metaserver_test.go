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

	err = client.AuthorizeRenter(rsaKey, renterID)
	if err != nil {
		return nil, errors.New("could not authorize")
	}

	return &renter, nil
}

func registerProvider(client *Client) (*core.ProviderInfo, error) {
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

	err = client.RegisterProvider(&provider)
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
	client := NewClient(core.DefaultMetaAddr, &httpClient)

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

// Delete renter
func TestDeleteRenter(t *testing.T) {
	httpClient := http.Client{}
	client := NewClient(core.DefaultMetaAddr, &httpClient)

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
// Get contracts
// Get contract
// Delete contract

// Register provider
func TestRegisterProvider(t *testing.T) {
	httpClient := http.Client{}
	client := NewClient(core.DefaultMetaAddr, &httpClient)

	provider, err := registerProvider(client)
	if err != nil {
		t.Fatal(err)
	}

	returned, err := client.GetProvider(provider.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(*provider, returned); diff != nil {
		t.Fatal(diff)
	}
}

// Update provider
func TestUpdateProvider(t *testing.T) {
	httpClient := http.Client{}
	client := NewClient(core.DefaultMetaAddr, &httpClient)

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

	if diff := deep.Equal(*provider, updatedProvider); diff != nil {
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
	client := NewClient(core.DefaultMetaAddr, &httpClient)

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
