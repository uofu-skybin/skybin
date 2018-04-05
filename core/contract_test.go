package core

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
	"time"
)

func TestSignVerify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	c1 := Contract{
		ID:           "cid",
		StartDate:    time.Now(),
		EndDate:      time.Now().Add(10 * time.Second),
		RenterId:     "abcdefg",
		ProviderId:   "hijklmnop",
		StorageSpace: 1024 * 1024,
		StorageFee:   1000000,
	}
	sig, err := SignContract(&c1, key)
	if err != nil {
		t.Fatal(err)
	}
	err = VerifyContractSignature(&c1, sig, key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate contract copies to check signature failure
	c2 := c1
	c2.ID = "1"
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}

	c2 = c1
	c2.RenterId = "1"
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}

	c2 = c1
	c2.ProviderId = "1"
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}

	c2 = c1
	c2.StorageSpace = 39834
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}

	c2 = c1
	c2.StorageFee = 99
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}

	c2 = c1
	c2.StartDate = time.Now().Add(1 * time.Minute)
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}

	c2 = c1
	c2.EndDate = time.Now().Add(1 * time.Minute)
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}
}

func TestCompare(t *testing.T) {
	c1 := Contract{
		ID:                "cid",
		RenterId:          "rid",
		StartDate:         time.Now(),
		EndDate:           time.Now().Add(10 * time.Second),
		ProviderId:        "pid",
		StorageSpace:      1 << 30,
		StorageFee:        1000000,
		RenterSignature:   "rsig",
		ProviderSignature: "psig",
	}
	c2 := c1
	if !CompareContracts(c1, c2) {
		t.Fatal("contracts should be the same")
	}
	c2.ID = "1"
	if CompareContracts(c1, c2) {
		t.Fatal("contracts should be different")
	}
	c2 = c1
	c2.RenterId = "1"
	if CompareContracts(c1, c2) {
		t.Fatal("contracts should be different")
	}
	c2 = c1
	c2.ProviderId = "1"
	if CompareContracts(c1, c2) {
		t.Fatal("contracts should be different")
	}
	c2 = c1
	c2.StorageSpace = 1
	if CompareContracts(c1, c2) {
		t.Fatal("contracts should be different")
	}
	c2 = c1
	c2.StorageFee = 99
	if CompareContracts(c1, c2) {
		t.Fatal("contracts should be different")
	}
	c2 = c1
	c1.RenterSignature = "1"
	if CompareContracts(c1, c2) {
		t.Fatal("contracts should be different")
	}
	c2 = c1
	c1.ProviderSignature = "1"
	if CompareContracts(c1, c2) {
		t.Fatal("contracts should be different")
	}
	c2 = c1
	c2.StartDate = time.Now().AddDate(2, 0, 0)
	if CompareContracts(c1, c2) {
		t.Fatal("contracts should be different")
	}
	c2 = c1
	c2.EndDate = time.Now().AddDate(2, 0, 0)
	if CompareContracts(c1, c2) {
		t.Fatal("contracts should be different")
	}
}

func TestCompareTerms(t *testing.T) {
	c1 := Contract{
		ID:                "cid",
		RenterId:          "dxzcvew",
		ProviderId:        "oiupbjn",
		StartDate:         time.Now(),
		EndDate:           time.Now().AddDate(0, 1, 0),
		StorageSpace:      3734,
		StorageFee:        99938234,
		RenterSignature:   "askdjou",
		ProviderSignature: "doueqnf",
	}
	c2 := c1
	doMatch := CompareContractTerms(&c1, &c2)
	if !doMatch {
		t.Fatal("identical contracts should match")
	}

	c2 = c1
	c2.RenterSignature = "cxmnv"
	c2.ProviderSignature = "c,mzoj"
	doMatch = CompareContractTerms(&c1, &c2)
	if !doMatch {
		t.Fatal("identical contracts with different signatures should match")
	}

	c2 = c1
	c2.RenterId = "1"
	doMatch = CompareContractTerms(&c1, &c2)
	if doMatch {
		t.Fatal("contracts should not match")
	}

	c2 = c1
	c2.ProviderId = "1"
	doMatch = CompareContractTerms(&c1, &c2)
	if doMatch {
		t.Fatal("contracts should not match")
	}

	c2 = c1
	c2.StorageSpace = 1
	doMatch = CompareContractTerms(&c1, &c2)
	if doMatch {
		t.Fatal("contracts should not match")
	}

	c2 = c1
	c2.StorageFee = 43
	doMatch = CompareContractTerms(&c1, &c2)
	if doMatch {
		t.Fatal("contracts should not match")
	}

	c2 = c1
	c2.StartDate = time.Now().AddDate(1, 1, 1)
	doMatch = CompareContractTerms(&c1, &c2)
	if doMatch {
		t.Fatal("contracts should not match")
	}

	c2 = c1
	c2.EndDate = time.Now().AddDate(2, 2, 2)
	doMatch = CompareContractTerms(&c1, &c2)
	if doMatch {
		t.Fatal("contracts should not match")
	}
}

func TestSerializeDeserialize(t *testing.T) {
	c1 := Contract{
		ID:                "cid",
		RenterId:          "renter",
		ProviderId:        "provider",
		StorageSpace:      int64(123456789),
		StorageFee:        392349234,
		StartDate:         time.Now().UTC().Round(0),
		EndDate:           time.Now().AddDate(0, 0, 30*6),
		RenterSignature:   "dkjadsf",
		ProviderSignature: "dlsjojifea",
	}

	data, err := json.Marshal(&c1)
	if err != nil {
		t.Fatal(err)
	}

	var c2 Contract
	err = json.Unmarshal(data, &c2)
	if err != nil {
		t.Fatal(err)
	}

	if !CompareContractTerms(&c1, &c2) {
		t.Fatal("terms should match")
	}
	if !CompareContracts(c1, c2) {
		t.Fatal("contracts should match")
	}
}
