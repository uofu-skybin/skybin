package util

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"
)

func TestMarshalKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	// Test private key
	data := MarshalPrivateKey(rsaKey)
	key2, err := UnmarshalPrivateKey(data)
	if err != nil {
		t.Fatal(err)
	}

	// Check signatures match
	msg := []byte("0123456789")
	h := sha256.Sum256(msg)
	sig1, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	sig2, err := rsa.SignPKCS1v15(rand.Reader, key2, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(sig1, sig2) != 0 {
		t.Fatal("signatures don't match")
	}

	// Test public key
	data, err = MarshalPublicKey(&rsaKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubKey, err := UnmarshalPublicKey(data)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the unmarshalled key can verify the signatures
	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, h[:], sig1)
	if err != nil {
		t.Fatal(err)
	}
}
