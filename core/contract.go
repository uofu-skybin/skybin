package core

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
)

// A contract without the signature fields,
// with other fields sorted by name.
type contractTerms struct {
	ID           string `json:"id"`
	ProviderId   string `json:"providerId"`
	RenterId     string `json:"renterId"`
	StorageSpace int64  `json:"storageSpace"`
}

func makeTerms(c *Contract) contractTerms {
	return contractTerms{
		ID:           c.ID,
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
// It does not mutate the contract; it merely returns the base64 encoded signature.
func SignContract(contract *Contract, key *rsa.PrivateKey) (string, error) {
	h, err := hashContract(contract)
	if err != nil {
		return "", err
	}
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), err
}

// VerifyContractSignature checks that a signature matches a contract using the given key.
// signature should be a base64 encoded signature returned by SignContract.
func VerifyContractSignature(contract *Contract, signature string, key rsa.PublicKey) error {
	h, err := hashContract(contract)
	if err != nil {
		return err
	}
	sb, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return err
	}
	return rsa.VerifyPKCS1v15(&key, crypto.SHA256, h, sb)
}

// CompareContracts returns whether two contracts are the same, that is,
// that they have the same fields.
func CompareContracts(c1, c2 Contract) bool {
	return c1 == c2
}

// CompareContractTerms returns whether the terms of two contracts match.
// The terms of a contract include all fields except the signatures.
func CompareContractTerms(c1, c2 *Contract) bool {
	return makeTerms(c1) == makeTerms(c2)
}
