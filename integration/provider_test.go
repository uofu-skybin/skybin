package integration

import (
	"testing"
	"skybin/core"
	"skybin/provider"
	"net/http"
	"skybin/renter"
	"io/ioutil"
	"os"
)

// This test expects that the renter and provider services are running
// at these addresses. Additionally, no other providers should be running.
const (
	testProviderAddr = core.DefaultPublicProviderAddr
	testRenterAddr = core.DefaultRenterAddr
)

func TestAudit_BadBlock(t *testing.T) {
	pvdr := provider.NewClient(testProviderAddr, &http.Client{})
	_, err := pvdr.AuditBlock("", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = pvdr.AuditBlock("askjdf", "adksjf", "alsk")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAudit(t *testing.T) {
	renterClient := renter.NewClient(testRenterAddr, &http.Client{})
	_, err := renterClient.ReserveStorage(2 * 1024 * 1024 * 1024)
	if err != nil {
		t.Fatal(err)
	}
	tempFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	f, err := renterClient.Upload(tempFile.Name(), "audit_test_file")
	if err != nil {
		t.Fatal(err)
	}
	renterInfo, err := renterClient.GetInfo()
	if err != nil {
		t.Fatal(err)
	}
	pvdr := provider.NewClient(testProviderAddr, &http.Client{})
	for _, block := range f.Versions[0].Blocks {
		if len(block.Audits) == 0 {
			t.Error("uploaded block has no audit info.\n" +
				"cannot test audit without block audits enabled")
		}
	}
	for _, block := range f.Versions[0].Blocks {
		for _, audit := range block.Audits {
			hashStr, err := pvdr.AuditBlock(renterInfo.ID, block.ID, audit.Nonce)
			if err != nil {
				t.Fatal(err)
			}
			if hashStr != audit.ExpectedHash {
				t.Fatal("provider performed audit incorrectly")
			}
		}
	}
}
