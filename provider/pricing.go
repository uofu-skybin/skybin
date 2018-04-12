package provider

import (
	"math/rand"
	"net/http"
	"skybin/metaserver"
	"time"
)

func (provider *Provider) pricingUpdateThread() {
	provider.logger.Println("starting pricing update thread with update frequency", PricingUpdateFreq.String())
	ticker := time.NewTicker(PricingUpdateFreq)
	for {
		select {
		case <-ticker.C:
			provider.logger.Println("updating storage rates")
			provider.updatePricing()
		case <-provider.doneCh:
			provider.logger.Println("pricing update thread shutting down")
		}
	}
}

func (provider *Provider) updatePricing() {
	switch provider.Config.PricingPolicy {
	case FixedPricingPolicy:
		return
	case PassivePricingPolicy:
		// TODO: implement
		fallthrough
	case AggressivePricingPolicy:
		metaService := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
		providers, err := metaService.GetProviders()
		if err != nil {
			provider.logger.Println("unable to update pricing policy. cannot fetch providers. error: ", err)
			return
		}
		if len(providers) == 0 {
			return
		}
		tot := int64(0)
		for _, pinfo := range providers {
			if pinfo.ID == provider.Config.ProviderID {
				continue
			}
			tot += pinfo.StorageRate
		}
		avg := tot / int64(len(providers))
		noise := 5 - rand.Intn(8)
		rate := avg + 3 + int64(noise)
		if rate < 0 {
			rate = 0
		}
		if rate < provider.Config.MinStorageRate {
			rate = provider.Config.MinStorageRate
		}
		if rate > provider.Config.MaxStorageRate {
			rate = provider.Config.MaxStorageRate
		}

		provider.logger.Println("network average storage rate:", avg)
		provider.logger.Println("updating storage rate to:", rate)

		provider.mu.Lock()
		provider.Config.StorageRate = rate
		provider.mu.Unlock()
		provider.UpdateMeta()
	default:
		provider.logger.Println("unrecognized pricing policy", provider.Config.PricingPolicy)
	}
}
