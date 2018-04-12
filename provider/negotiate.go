package provider

import (
	"errors"
	"fmt"
	"skybin/core"
)

// Helper to negotiate contracts
func (provider *Provider) NegotiateContract(contract *core.Contract) (*core.Contract, error) {
	renterKey, err := provider.getRenterPublicKey(contract.RenterId)
	if err != nil {
		return nil, fmt.Errorf("Failed to get Renters pubkey from metaserver. error: %s", err)
	}

	// Verify renters signature
	err = core.VerifyContractSignature(contract, contract.RenterSignature, *renterKey)
	if err != nil {
		return nil, fmt.Errorf("Invalid Renter signature: %s", err)
	}

	// Determine if provider has sufficient space available for the contract
	avail := provider.Config.SpaceAvail - provider.StorageReserved
	if contract.StorageSpace > avail {
		return nil, errors.New("Provider does not have sufficient storage available")
	}

	// Sign contract
	provSig, err := core.SignContract(contract, provider.privKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to sign contract. error: %s", err)
	}
	contract.ProviderSignature = provSig

	// if renter isn't in set, add a new entry
	renter, exists := provider.renters[contract.RenterId]
	if !exists {
		renter = &renterInfo{}
		provider.renters[contract.RenterId] = renter
	}
	// Add storage space to the renter
	renter.StorageReserved += contract.StorageSpace

	err = provider.db.InsertContract(contract)
	if err != nil {
		return nil, fmt.Errorf("Failed to insert contract into DB. error: %s", err)
	}

	// activity updates are non-fatal errors
	err = provider.addActivity("contract", contract.StorageSpace)
	if err != nil {
		fmt.Println("Failed to update activity for contract: ", err)
	}

	// this could potentially be non-fatal too
	err = provider.UpdateMeta()
	if err != nil {
		return nil, fmt.Errorf("Error updating metaserver: %s", err)
	}

	return contract, nil
}
