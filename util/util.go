package util

import (
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base32"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
)

func Hash(data []byte) string {
	h := sha1.New()
	h.Write(data)
	return base32.StdEncoding.EncodeToString(h.Sum(nil))
}

func SaveJson(filename string, v interface{}) error {
	bytes, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, bytes, 0666)
}

func LoadJson(filename string, v interface{}) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, v)
}

func MarshalPrivateKey(key *rsa.PrivateKey) []byte {
	block := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(&block)
}

func UnmarshalPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("Expected PEM formatted key bytes.")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func MarshalPublicKey(key *rsa.PublicKey) ([]byte, error) {
	bytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	block := pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: bytes,
	}
	return pem.EncodeToMemory(&block), nil
}

func UnmarshalPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("Expected PEM formated key bytes.")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("Key is not an RSA public key.")
	}
	return key, nil
}
