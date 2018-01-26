package core

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
)

// A contract without the signature fields,
// with other fields sorted by name.
type contractTerms struct {
	ProviderId   string `json:"providerId"`
	RenterId     string `json:"renterId"`
	StorageSpace int64  `json:"storageSpace"`
}

func makeTerms(c *Contract) contractTerms {
	return contractTerms{
		ProviderId:   c.ProviderId,
		RenterId:     c.RenterId,
		StorageSpace: c.StorageSpace,
	}
}

func hashContract(contract *Contract) ([]byte, error) {
	p := makeTerms(contract)
	data, err := json.Marshal(&p)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256(data)
	return h[:], nil
}

// SignContract signs a contract with a given key.
// It does not mutate the contract; it merely returns the signature.
func SignContract(contract *Contract, key *rsa.PrivateKey) ([]byte, error) {
	h, err := hashContract(contract)
	if err != nil {
		return nil, err
	}
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		return nil, err
	}
	return signature, err
}

// VerifyContractSignature checks that a signature matches a contract using the given key.
func VerifyContractSignature(contract *Contract, signature []byte, key rsa.PublicKey) error {
	h, err := hashContract(contract)
	if err != nil {
		return err
	}
	return rsa.VerifyPKCS1v15(&key, crypto.SHA256, h, signature)
}

// CompareContractTerms returns whether the terms of two contracts match.
// The terms of a contract include all fields except the signatures.
func CompareContractTerms(c1, c2 *Contract) bool {
	return makeTerms(c1) == makeTerms(c2)
}
