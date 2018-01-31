package metaserver

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
)

func parsePublicKey(key string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(key))
	if block == nil {
		return nil, errors.New("could not decode PEM")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid public key")
	}

	if publicKey.(*rsa.PublicKey) == nil {
		return nil, errors.New("key is not a public key")
	}

	return publicKey.(*rsa.PublicKey), nil
}

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
