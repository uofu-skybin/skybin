package renter

import (
	"errors"
	"skybin/core"
	"skybin/metaserver"
	"net/http"
	"fmt"
	"skybin/provider"
	"math/rand"
)

func (r *Renter) ReserveStorage(amount int64) ([]*core.Contract, error) {
	metaService := metaserver.NewClient(r.Config.MetaAddr, &http.Client{})
	providers, err := metaService.GetProviders()
	if err != nil {
		return nil, fmt.Errorf("Cannot fetch providers. Error: %v", err)
	}

	var reserved int64 = 0
	var contracts []*core.Contract
	var blobs []*storageBlob
	for reserved < amount {
		n := amount - reserved
		if n > kMaxContractSize {
			n = kMaxContractSize
		}
		contract, pinfo, err := r.reserveN(n, providers)
		if err != nil {
			return nil, fmt.Errorf("Cannot find enough storage. Error: %v", err)
		}
		contracts = append(contracts, contract)
		blobs = append(blobs, &storageBlob{
			ProviderId: pinfo.ID,
			Addr: pinfo.Addr,
			Amount: contract.StorageSpace,
			ContractId: contract.ID,
		})
		reserved += n
	}

	r.contracts = append(r.contracts, contracts...)
	r.freelist = append(r.freelist, blobs...)
	err = r.saveSnapshot()
	if err != nil {
		return nil, err
	}

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

	// Get an updated version of their information and double-check
	// that they have enough space.
	//pinfo, err := client.GetInfo()
	//if err != nil {
	//	return nil, err
	//}
	//if pinfo.SpaceAvail < space {
	//	return nil, errors.New("Provider doesn't have enough space")
	//}

	cid, err := genId()
	if err != nil {
		return nil, err
	}
	proposal := core.Contract{
		ID: cid,
		RenterId: r.Config.RenterId,
		ProviderId: pinfo.ID,
		StorageSpace: space,
		RenterSignature: "signature",
	}

	signedContract, err := client.ReserveStorage(&proposal)
	if err != nil {
		return nil, err
	}
	if len(signedContract.ProviderSignature) == 0 {
		return nil, errors.New("Provider did not agree to contract")
	}

	return signedContract, nil
}
