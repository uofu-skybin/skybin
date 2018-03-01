package provider

import (
	"errors"
	"fmt"
	"skybin/core"
)

func (provider *Provider) NegotiateContract(contract *core.Contract) (*core.Contract, error) {
	renterKey, err := provider.getRenterPublicKey(contract.RenterId)
	if err != nil {
		return nil, errors.New("Metadata server does not have an associated renter ID")
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
	provSig, err := core.SignContract(contract, provider.PrivateKey)
	if err != nil {
		return nil, errors.New("Error signing contract")
	}
	contract.ProviderSignature = provSig

	// Add storage space to the renter
	renter, exists := provider.renters[contract.RenterId]
	if !exists {
		renter = &RenterInfo{
			Contracts: []*core.Contract{},
			Blocks:    []*BlockInfo{},
		}
		provider.renters[contract.RenterId] = renter
	}
	renter.StorageReserved += contract.StorageSpace
	renter.Contracts = append(renter.Contracts, contract)
	provider.StorageReserved += contract.StorageSpace
	provider.contracts = append(provider.contracts, contract)

	// provider.addStat("contract", contract.StorageSpace)
	// activity := Activity{
	// 	RequestType: negotiateType,
	// 	Contract:    contract,
	// 	TimeStamp:   time.Now(),
	// 	RenterId:    contract.RenterId,
	// }
	// provider.addActivity(activity)

	err = provider.saveSnapshot()
	if err != nil {

		// TODO: Remove contract. I don't do this here
		// since we need to move to an improved storage scheme anyways.
		return nil, fmt.Errorf("Unable to save contract. Error: %s", err)
	}
	return contract, nil
}
