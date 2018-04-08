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
	"crypto/rsa"
	"math"
)

type storageEstimate struct {
	// Total space the estimate reserves in bytes
	TotalSpace int64 `json:"totalSpace"`
	// Total price in tenths of cents
	TotalPrice int64 `json:"totalPrice"`
	// Tentative contracts. Terms are set but signatures are empty
	Contracts []*core.Contract `json:"contracts"`
	// Providers the contracts are with. Contracts[i] is with Providers[i]
	Providers []*core.ProviderInfo `json:"providers"`
}

type pvdrDialFn func(*core.ProviderInfo) pvdrIface

func dialProvider(pvdr *core.ProviderInfo) pvdrIface {
	return provider.NewClient(pvdr.Addr, &http.Client{})
}

type pvdrIface interface {
	GetInfo() (*core.ProviderInfo, error)
	ReserveStorage(contract *core.Contract) (*core.Contract, error)
}

func (r *Renter) ReserveStorage(totalSpace int64) ([]*core.Contract, error) {
	if totalSpace < kMinContractSize {
		return nil, fmt.Errorf("Must reserve at least %d bytes.", kMinContractSize)
	}
	err := r.authorizeMeta()
	if err != nil {
		return nil, err
	}
	providers, err := r.metaClient.GetProviders()
	if err != nil {
		return nil, fmt.Errorf("Cannot fetch providers. Error: %v", err)
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("Cannot find any storage providers.")
	}
	estimate, err := createStorageEstimate(totalSpace, r.Config, providers, dialProvider)
	if err != nil {
		return nil, fmt.Errorf("Unable to find enough storage space.")
	}
	err = confirmStorageEstimate(estimate, r.privKey, dialProvider)
	if err != nil {
		return nil, err
	}

	// Save the contracts
	for _, contract := range estimate.Contracts {
		err = r.metaClient.PostContract(r.Config.RenterId, contract)
		if err != nil {
			return nil, err
		}
	}
	r.contracts = append(r.contracts, estimate.Contracts...)

	// Record the new storage blobs
	blobs := []*storageBlob{}
	for i := 0; i < len(estimate.Contracts); i++ {
		contract := estimate.Contracts[i]
		pinfo := estimate.Providers[i]
		blobs = append(blobs, &storageBlob{
			ProviderId: pinfo.ID,
			Addr:       pinfo.Addr,
			Amount:     contract.StorageSpace,
			ContractId: contract.ID,
		})
	}
	r.storageManager.AddBlobs(blobs)
	return estimate.Contracts, nil
}

func createStorageEstimate(totalSpace int64, config *Config,
	providers []core.ProviderInfo,
	dialFn pvdrDialFn) (*storageEstimate, error) {

	estimate := &storageEstimate{}
	startDate := time.Now().UTC().Round(0)
	endDate := time.Now().AddDate(0, 0, config.DefaultContractDurationDays)
	badPvdrs := make([]bool, len(providers))
	spaceLeft := make([]int64, len(providers))
	visited := make([]bool, len(providers))
	pvdrsLeft := len(providers)

	for estimate.TotalSpace < totalSpace && pvdrsLeft > 0 {
		space := totalSpace - estimate.TotalSpace
		if space > config.MaxContractSize {
			space = config.MaxContractSize
		}
		for pvdrsLeft > 0 {
			idx := rand.Intn(len(providers))
			if badPvdrs[idx] {
				continue
			}
			pinfo := &providers[idx]
			if pinfo.SpaceAvail < space {
				badPvdrs[idx] = true
				pvdrsLeft--
				continue
			}
			if !visited[idx] {
				// We haven't visited this provider yet.
				// Pull a refreshed copy of the provider's info.
				// This does two things:
				//   1) It ensures the provider is online
				//   2) It ensures we have up-to-date information on the provider's space and fees
				client := dialFn(pinfo)
				var err error
				pinfo, err = client.GetInfo()
				if err != nil {
					badPvdrs[idx] = true
					pvdrsLeft--
					continue
				}
				spaceLeft[idx] = pinfo.SpaceAvail
				providers[idx] = *pinfo
				visited[idx] = true
			}
			if spaceLeft[idx] < space {
				badPvdrs[idx] = true
				pvdrsLeft--
				continue
			}
			spaceLeft[idx] -= space
			fee := calcStorageFee(space, int64(config.DefaultContractDurationDays), pinfo.StorageRate)
			cid, err := genId()
			if err != nil {
				return nil, err
			}
			proposal := &core.Contract{
				ID:           cid,
				StartDate:    startDate,
				EndDate:      endDate,
				RenterId:     config.RenterId,
				ProviderId:   pinfo.ID,
				StorageSpace: space,
				StorageFee:   fee,
			}
			estimate.Contracts = append(estimate.Contracts, proposal)
			estimate.Providers = append(estimate.Providers, pinfo)
			estimate.TotalSpace += space
			estimate.TotalPrice += fee
			break
		}
	}
	if estimate.TotalSpace != totalSpace {
		return nil, errors.New("Unable to find enough storage space.")
	}
	return estimate, nil
}

func calcStorageFee(spaceBytes, durationDays, rateGbMonth int64) int64 {
	spaceGb := float64(spaceBytes) / float64(1e9)
	durationMonths := float64(durationDays) / float64(30)
	z := int64(math.Ceil(spaceGb * durationMonths))
	return z * rateGbMonth
}

func confirmStorageEstimate(estimate *storageEstimate, signingKey *rsa.PrivateKey, dialFn pvdrDialFn) error {
	for i := 0; i < len(estimate.Contracts); i++ {
		contract := estimate.Contracts[i]
		pinfo := estimate.Providers[i]
		pvdrKey, err := util.UnmarshalPublicKey([]byte(pinfo.PublicKey))
		if err != nil {
			return errors.New("Unable to unmarshal provider's key")
		}
		signature, err := core.SignContract(contract, signingKey)
		if err != nil {
			return fmt.Errorf("Unable to sign contract. Error: %s", err)
		}
		contract.RenterSignature = signature

		client := dialFn(pinfo)
		signedContract, err := client.ReserveStorage(contract)
		if err != nil {
			return err
		}
		if len(signedContract.ProviderSignature) == 0 {
			return errors.New("Provider did not agree to contract")
		}
		err = core.VerifyContractSignature(signedContract, signedContract.ProviderSignature, *pvdrKey)
		if err != nil {
			return errors.New("Provider's signature does not match contract")
		}
		contract.ProviderSignature = signedContract.ProviderSignature
		if !core.CompareContracts(*contract, *signedContract) {
			return errors.New("Provider's terms don't match original contract")
		}
	}
	return nil
}

