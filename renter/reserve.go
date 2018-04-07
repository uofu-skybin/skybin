package renter

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"skybin/core"
	"skybin/provider"
	"skybin/util"
	"time"
)

func (r *Renter) ReserveStorage(amount int64) ([]*core.Contract, error) {
	if amount < kMinContractSize {
		return nil, fmt.Errorf("Must reserve at least %d bytes.", kMinContractSize)
	}

	providers, err := r.metaClient.GetProviders()
	if err != nil {
		return nil, fmt.Errorf("Cannot fetch providers. Error: %v", err)
	}

	var reserved int64 = 0
	var contracts []*core.Contract
	var blobs []*storageBlob
	for reserved < amount {
		n := amount - reserved
		if n > r.Config.MaxContractSize {
			n = r.Config.MaxContractSize
		}
		contract, pinfo, err := r.reserveN(n, providers)
		if err != nil {
			return nil, fmt.Errorf("Cannot find enough storage. Error: %v", err)
		}
		contracts = append(contracts, contract)
		blobs = append(blobs, &storageBlob{
			ProviderId: pinfo.ID,
			Addr:       pinfo.Addr,
			Amount:     contract.StorageSpace,
			ContractId: contract.ID,
		})
		reserved += n
	}

	// Save contracts with metaserver.
	// TODO: Use batch save endpoint.
	err = r.authorizeMeta()
	if err != nil {
		return nil, err
	}
	for _, contract := range contracts {
		err = r.metaClient.PostContract(r.Config.RenterId, contract)
		if err != nil {
			return nil, err
		}
	}

	// Record the newly available storage locally
	r.storageManager.AddBlobs(blobs)

	return contracts, nil
}

func (r *Renter) reserveN(n int64, providers []core.ProviderInfo) (*core.Contract, *core.ProviderInfo, error) {
	const randiters = 1024
	visited := make([]bool, len(providers))
	nvisited := 0
	for i := 0; i < randiters && nvisited < len(providers); i++ {
		idx := rand.Intn(len(providers))
		if visited[idx] {
			continue
		}
		visited[idx] = true
		nvisited++
		pinfo := providers[idx]
		if pinfo.SpaceAvail < n {
			continue
		}
		contract, err := r.formContract(n, &pinfo)
		if err != nil {
			continue
		}
		return contract, &pinfo, nil
	}
	return nil, nil, errors.New("Cannot find provider")
}

func (r *Renter) formContract(space int64, pinfo *core.ProviderInfo) (*core.Contract, error) {
	client := provider.NewClient(pinfo.Addr, &http.Client{})

	// Get an updated version of the provider's information and double-check
	// that they have enough space.
	pinfo, err := client.GetInfo()
	if err != nil {
		return nil, err
	}
	if pinfo.SpaceAvail < space {
		return nil, errors.New("Provider doesn't have enough space")
	}

	// Check that the provider's key is valid.
	pvdrKey, err := util.UnmarshalPublicKey([]byte(pinfo.PublicKey))
	if err != nil {
		return nil, errors.New("Unable to unmarshal provider's key")
	}

	// Create proposal
	cid, err := genId()
	if err != nil {
		return nil, err
	}
	proposal := core.Contract{
		ID:           cid,
		StartDate:    time.Now().UTC().Round(0),
		EndDate:      time.Now().AddDate(0, 0, r.Config.DefaultContractDurationDays),
		RenterId:     r.Config.RenterId,
		ProviderId:   pinfo.ID,
		StorageSpace: space,
	}
	signature, err := core.SignContract(&proposal, r.privKey)
	if err != nil {
		return nil, fmt.Errorf("Unable to sign contract. Error: %s", err)
	}
	proposal.RenterSignature = signature

	// Send to provider
	signedContract, err := client.ReserveStorage(&proposal)
	if err != nil {
		return nil, err
	}

	if len(signedContract.ProviderSignature) == 0 {
		return nil, errors.New("Provider did not agree to contract")
	}

	err = core.VerifyContractSignature(signedContract, signedContract.ProviderSignature, *pvdrKey)
	if err != nil {
		return nil, errors.New("Provider's signature does not match contract")
	}

	proposal.ProviderSignature = signedContract.ProviderSignature
	if !core.CompareContracts(proposal, *signedContract) {
		return nil, errors.New("Provider's terms don't match original contract")
	}

	return signedContract, nil
}
