package renter

import (
	"errors"
	"skybin/core"
	"testing"
	"math/rand"
)

type mockProvider struct {
	pinfo *core.ProviderInfo
}

func (mp *mockProvider) GetInfo() (*core.ProviderInfo, error) {
	return mp.pinfo, nil
}

func (mp *mockProvider) ReserveStorage(contract *core.Contract) (*core.Contract, error) {
	return nil, errors.New("not implemented")
}

func testDialFn(pinfo *core.ProviderInfo) pvdrIface {
	return &mockProvider{pinfo}
}

func TestCreateStorageEstimate_SingleProvider(t *testing.T) {
	config := Config{
		RenterId:                    "r1",
		MaxContractSize:             1024,
		DefaultContractDurationDays: 60,
	}
	providers := []core.ProviderInfo{
		{
			SpaceAvail:  1024,
		},
	}
	_, err := createStorageEstimate(1024, &config, providers, testDialFn)
	if err != nil {
		t.Fatal("failed to reserve storage. error: ", err)
	}
}

func TestCreateStorageEstimate_SmallContractSize(t *testing.T) {
	config := Config{
		RenterId:                    "r1",
		MaxContractSize:             512,
		DefaultContractDurationDays: 60,
	}
	providers := []core.ProviderInfo{
		{
			SpaceAvail:  1024*4,
		},
	}
	_, err := createStorageEstimate(1024*4, &config, providers, testDialFn)
	if err != nil {
		t.Fatal("failed to reserve storage. error: ", err)
	}
}

func TestCreateStorageEstimate_OnlyOneGoodProvider(t *testing.T) {
	config := Config{
		RenterId:                    "r1",
		MaxContractSize:             1000,
		DefaultContractDurationDays: 60,
	}
	providers := []core.ProviderInfo{
		{
			SpaceAvail:  4000,
		},
	}
	for i := 0; i < 100; i++ {
		providers = append(providers, core.ProviderInfo{
			SpaceAvail: 50,
		})
	}
	_, err := createStorageEstimate(4000, &config, providers, testDialFn)
	if err != nil {
		t.Fatal("failed to reserve storage. error: ", err)
	}
}

func TestCreateStorageEstimate_NotEnoughStorage(t *testing.T) {
	config := Config{
		RenterId:                    "r1",
		MaxContractSize:             1024,
		DefaultContractDurationDays: 60,
	}
	providers := []core.ProviderInfo{
		{
			SpaceAvail: 1024,
		},
		{
			SpaceAvail: 1024,
		},
		{
			SpaceAvail: 1024,
		},
	}
	_, err := createStorageEstimate(10000, &config, providers, testDialFn)
	if err == nil {
		t.Fatal("created storage estimate without enough storage")
	}
}

func createStorageFuzz(t *testing.T) {
	config := Config{
		RenterId:                    "r1",
		MaxContractSize:             1024,
		DefaultContractDurationDays: 30,
	}
	providers := []core.ProviderInfo{}
	nproviders := rand.Intn(500)
	usableSpace := 0
	for i := 0; i < nproviders; i++ {
		pvdr := core.ProviderInfo{
			SpaceAvail: int64(1024 * rand.Intn(4) + rand.Intn(1024)),
		}
		usableSpace += int((pvdr.SpaceAvail / 1024) * 1024)
		providers = append(providers, pvdr)
	}
	spaceToReserve := int64(rand.Intn(usableSpace/2) + usableSpace/2)
	_, err := createStorageEstimate(spaceToReserve, &config, providers, testDialFn)
	if err != nil {
		t.Fatal("failed to reserve storage. error: ", err)
	}
}

func TestCreateStorageEstimate_Fuzz(t *testing.T) {
	for i := 0; i < 10; i++ {
		createStorageFuzz(t)
	}
}

func TestCalcStorageFee(t *testing.T) {
	check := func(got, expected int64) {
		if got != expected {
			t.Fatal("wrong fee. got", got, "expected", expected)
		}
	}
	check(calcStorageFee(1e9, 30, 0), 0)
	check(calcStorageFee(1, 1, 1), 1)
	check(calcStorageFee(1e9, 30, 1), 1)
	check(calcStorageFee(8*1e9, 60, 1), 8*2)
	check(calcStorageFee(1e9/2, 60, 5), 5)
	check(calcStorageFee(1e9/4, 45, 8), 8)
	check(calcStorageFee(1e9, 15, 1), 1)
	check(calcStorageFee(1e9, 45, 1), 2)
}
