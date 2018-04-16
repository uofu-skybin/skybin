package provider

import (
	"math/rand"
	"net/http"
	"skybin/core"
	"skybin/metaserver"
	"sort"
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
	if provider.Config.PricingPolicy == FixedPricingPolicy {
		return
	}
	metaService := metaserver.NewClient(provider.Config.MetaAddr, &http.Client{})
	providers, err := metaService.GetProviders()
	if err != nil {
		provider.logger.Println("unable to update pricing policy. cannot fetch providers. error: ", err)
		return
	}
	if len(providers) == 0 {
		return
	}

	rates := []int64{}
	for _, pinfo := range providers {
		if pinfo.ID != provider.Config.ProviderID {
			rates = append(rates, pinfo.StorageRate)
		}
	}

	sort.Slice(rates, func(i, j int) bool {
		return rates[i] < rates[j]
	})

	var rate int64
	var avg int64

	switch provider.Config.PricingPolicy {
	case AggressivePricingPolicy:
		// Set the price to the average of the most expensive quartile of providers.
		mostExpensiveQuartile := len(rates) - len(rates) / 4
		if mostExpensiveQuartile == len(rates) {
			mostExpensiveQuartile -= 1
		}
		if mostExpensiveQuartile < 0 {
			mostExpensiveQuartile = 0
		}

		tot := int64(0)
		for i := mostExpensiveQuartile; i < len(rates); i++ {
			tot += rates[i]
		}
		avg = tot
		if l := int64(len(rates) - mostExpensiveQuartile); l > 0 {
			avg /= l
		}
		noise := 3 - rand.Intn(7)
		rate = avg + int64(noise)
	case PassivePricingPolicy:
		// Set the price to the average of the middle fifty percent of providers.
		cheapestQuartile := len(rates) / 4
		mostExpensiveQuartile := len(rates) - cheapestQuartile

		tot := int64(0)
		for i := cheapestQuartile; i < mostExpensiveQuartile; i++ {
			tot += rates[i]
		}
		avg = tot
		if l := int64(mostExpensiveQuartile-cheapestQuartile); l > 0 {
			avg /= l
		}
		noise := 3 - rand.Intn(7)
		rate = avg + int64(noise)
	default:
		provider.logger.Println("unrecognized pricing policy", provider.Config.PricingPolicy)
	}

	if rate < 0 {
		rate = 0
	}
	if rate < provider.Config.MinStorageRate {
		rate = provider.Config.MinStorageRate
	}
	if rate > provider.Config.MaxStorageRate {
		rate = provider.Config.MaxStorageRate
	}

	provider.logger.Println("average storage rate:", avg)
	provider.logger.Println("updating storage rate to:", rate)

	provider.mu.Lock()
	provider.Config.StorageRate = rate
	provider.mu.Unlock()
	provider.UpdateMeta()
}
