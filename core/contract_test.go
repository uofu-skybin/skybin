package core

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
)

func TestSignVerify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	c := Contract{
		RenterId:     "abcdefg",
		ProviderId:   "hijklmnop",
		StorageSpace: 1024 * 1024,
	}
	sig, err := SignContract(&c, key)
	if err != nil {
		t.Fatal(err)
	}
	err = VerifyContractSignature(&c, sig, key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate contract copies to check signature failure
	c2 := c
	c2.RenterId = "1"
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}

	c2 = c
	c2.ProviderId = "1"
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}

	c2 = c
	c2.StorageSpace = 39834
	err = VerifyContractSignature(&c2, sig, key.PublicKey)
	if err == nil {
		t.Fatal("verify should fail - contract does not match original")
	}
}

func TestCompareTerms(t *testing.T) {
	c1 := Contract{
		RenterId:          "dxzcvew",
		ProviderId:        "oiupbjn",
		StorageSpace:      3734,
		RenterSignature:   "askdjou",
		ProviderSignature: "doueqnf",
	}
	c2 := c1
	doMatch := CompareContractTerms(&c1, &c2)
	if !doMatch {
		t.Fatal("identical contracts should match")
	}

	// Change signatures
	c2 = c1
	c2.RenterSignature = "cxmnv"
	c2.ProviderSignature = "c,mzoj"
	doMatch = CompareContractTerms(&c1, &c2)
	if !doMatch {
		t.Fatal("identical contracts with different signatures should match")
	}

	// Change renter ID
	c2 = c1
	c2.RenterId = "1"
	doMatch = CompareContractTerms(&c1, &c2)
	if doMatch {
		t.Fatal("contracts should not match")
	}

	// Change provider ID
	c2 = c1
	c2.ProviderId = "1"
	doMatch = CompareContractTerms(&c1, &c2)
	if doMatch {
		t.Fatal("contracts should not match")
	}

	// Change storage space
	c2 = c1
	c2.StorageSpace = 1
	doMatch = CompareContractTerms(&c1, &c2)
	if doMatch {
		t.Fatal("contracts should not match")
	}
}
