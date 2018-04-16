package provider

import (
	"errors"
	"fmt"
	"skybin/core"
)

func (provider *Provider) NegotiateContract(contract *core.Contract) (*core.Contract, error) {

	// TODO: remove this call
	renterKey, err := provider.getRenterPublicKey(contract.RenterId)
	if err != nil {
		return nil, fmt.Errorf("Failed to get Renters pubkey from metaserver. error: %s", err)
	}

	// Determine if provider has sufficient space available for the contract
	provider.mu.RLock()
	spaceAvail := provider.Config.SpaceAvail - provider.StorageReserved
	provider.mu.RUnlock()
	if contract.StorageSpace > spaceAvail {
		return nil, errors.New("Provider does not have sufficient storage available")
	}

	err = core.VerifyContractSignature(contract, contract.RenterSignature, *renterKey)
	if err != nil {
		return nil, fmt.Errorf("Invalid Renter signature: %s", err)
	}

	provSig, err := core.SignContract(contract, provider.privKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to sign contract. error: %s", err)
	}
	contract.ProviderSignature = provSig

	err = provider.db.InsertContract(contract)
	if err != nil {
		return nil, fmt.Errorf("Failed to insert contract into DB. error: %s", err)
	}

	provider.mu.Lock()
	renter, exists := provider.renters[contract.RenterId]
	if !exists {
		renter = &renterInfo{}
		provider.renters[contract.RenterId] = renter
	}
	renter.StorageReserved += contract.StorageSpace
	provider.StorageReserved += contract.StorageSpace
	provider.mu.Unlock()

	// this could potentially be non-fatal too
	err = provider.UpdateMeta()
	if err != nil {
		return nil, fmt.Errorf("Error updating metaserver: %s", err)
	}

	err = provider.addActivity(activityOpContract, contract.StorageSpace)
	if err != nil {
		// non-fatal
		fmt.Println("Failed to update activity for contract: ", err)
	}

	return contract, nil
}
