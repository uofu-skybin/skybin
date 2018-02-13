package util

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"
	"strings"
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

func TestUnMarshalInvalidKey(t *testing.T) {
	badKeys := []string{"", "not a key", strings.Repeat("a", 3000)}
	for _, badKey := range badKeys {
		_, err := UnmarshalPrivateKey([]byte(badKey))
		if err == nil {
			t.Fatalf("expected error unmarshalling invalid private key %s", badKey)
		}
		_, err = UnmarshalPublicKey([]byte(badKey))
		if err == nil {
			t.Fatalf("expected error unmarshalling invalid public key %s", badKey)
		}
	}
}

func TestParseByteAmountInvalid(t *testing.T) {
	var err error
	_, err = ParseByteAmount("not a number gb")
	if err == nil {
		t.Fatal("parsed invalid amount")
	}
	_, err = ParseByteAmount("10000000000000000000000000000000000000 TB")
	if err == nil {
		t.Fatal("allowed too large of amount")
	}
	_, err = ParseByteAmount("10 NT")
	if err == nil {
		t.Fatal("allowed invalid prefix")
	}
	_, err = ParseByteAmount("10.532 B")
	if err == nil {
		t.Fatal("allowed fractional byte amount")
	}
}

func TestParseByteAmount(t *testing.T) {
	var amt int64
	var err error

	amt, err = ParseByteAmount("10 tb")
	if err != nil {
		t.Fatal(err)
	}
	if amt != int64(10) * int64(1e12) {
		t.Fatal("wrong amount for TB suffix")
	}

	amt, err = ParseByteAmount("1 gb")
	if err != nil {
		t.Fatal(err)
	}
	if amt != int64(1e9) {
		t.Fatal("wrong amount for GB suffix")
	}

	amt, err = ParseByteAmount("198 MB")
	if err != nil {
		t.Fatal(err)
	}
	if amt != int64(198) * int64(1e6) {
		t.Fatal("wrong amount for MB suffix")
	}

	amt, err = ParseByteAmount("928 KB")
	if err != nil {
		t.Fatal(err)
	}
	if amt != int64(928 * 1000) {
		t.Fatal("wrong amount for KB suffix")
	}

	amt, err = ParseByteAmount("42 B")
	if err != nil {
		t.Fatal(err)
	}
	if amt != int64(42) {
		t.Fatal("wrong amount for B suffix")
	}

	amt, err = ParseByteAmount(" 12   ")
	if err != nil {
		t.Fatal(err)
	}
	if amt != int64(12) {
		t.Fatal("wrong amount with leading/trailing whitespace")
	}

	amt, err = ParseByteAmount(" 1398 KB")
	if err != nil {
		t.Fatal(err)
	}
	if amt != int64(1398 * 1000) {
		t.Fatal("wrong amount with > 1000 of a unit")
	}

	amt, err = ParseByteAmount(" -43")
	if err != nil {
		t.Fatal(err)
	}
	if amt != int64(-43) {
		t.Fatal("wrong amount for negative bytes")
	}
}

func TestValidateNetAddr(t *testing.T) {
	var err error

	err = ValidateNetAddr("")
	if err == nil {
		t.Fatal("allowed empty addr")
	}
	err = ValidateNetAddr("not valid")
	if err == nil {
		t.Fatal("allowed invalid addr")
	}
	err = ValidateNetAddr(":3000")
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateNetAddr("localhost:4000")
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateNetAddr("127.0.0.1:4000")
	if err != nil {
		t.Fatal(err)
	}
}
