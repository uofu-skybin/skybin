package metaserver

import (
	"bytes"
	"crypto/md5"
	"crypto/rsa"
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

func fingerprintKey(key string) (string, error) {
	md5Sum := md5.Sum([]byte(key))
	fingerprintNoColons := hex.EncodeToString(md5Sum[:])
	var buf bytes.Buffer
	for i, item := range fingerprintNoColons {
		if i%2 == 0 && i != 0 {
			buf.WriteString(":")
		}
		buf.WriteString(string(item))
	}

	return buf.String(), nil
}
