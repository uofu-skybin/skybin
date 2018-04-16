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

type ByRate []core.ProviderInfo

func (r ByRate) Len() int           { return len(r) }
func (r ByRate) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ByRate) Less(i, j int) bool { return r[i].StorageRate < r[j].StorageRate }

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

	// Set rate to the average of all the providers in the cheapest quartile.
	sort.Sort(ByRate(providers))

	var rate int64
	var avg int64

	switch provider.Config.PricingPolicy {
	case AggressivePricingPolicy:
		// Set the price to the average of the cheapest quartile of providers.
		cheapestQuartile := len(providers) / 4

		// If the cheapest quartile doesn't exist, set the price to the average of all current providers.
		if cheapestQuartile == 0 {
			cheapestQuartile = len(providers) - 1
		}

		tot := int64(0)
		for i := 0; i < cheapestQuartile; i++ {
			pinfo := providers[i]
			if pinfo.ID == provider.Config.ProviderID {
				continue
			}
			tot += pinfo.StorageRate
		}
		avg = tot / int64(cheapestQuartile)
		noise := 3 - rand.Intn(5)
		rate = avg + 3 + int64(noise)

	case PassivePricingPolicy:
		// Set the price to the average of the middle fifty percent of providers.
		cheapestQuartile := len(providers) / 4
		mostExpensiveQuartile := len(providers) / 4 * 3

		// If we don't have enough providers to exclude the cheapest and most expensive quartiles,
		// take the average of the entire group.
		if cheapestQuartile == mostExpensiveQuartile {
			cheapestQuartile = 0
			mostExpensiveQuartile = len(providers)
		}

		tot := int64(0)
		for i := cheapestQuartile; i < mostExpensiveQuartile; i++ {
			pinfo := providers[i]
			if pinfo.ID == provider.Config.ProviderID {
				continue
			}
			tot += pinfo.StorageRate
		}
		avg = tot / int64(mostExpensiveQuartile-cheapestQuartile)
		noise := 3 - rand.Intn(5)
		rate = avg + 3 + int64(noise)
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
