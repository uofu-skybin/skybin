package renter

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func noOpUpdateFn() ([]*storageBlob, error) {
	return nil, errors.New("")
}

type mockClock struct {
	nextTime time.Time
}

func (mc *mockClock) Now() time.Time {
	return mc.nextTime
}

func TestAvailableStorage(t *testing.T) {
	blobs := []*storageBlob{
		{
			ProviderId: "p1",
			Amount:     100,
		},
		{
			ProviderId: "p1",
			Amount:     100,
		},
	}
	sm := newStorageManager(blobs, noOpUpdateFn, time.Minute, &realClock{})
	if sm.AvailableStorage() != 200 {
		t.Fatal("wrong amount of available storage")
	}
	_, err := sm.FindStorage(1, 100)
	if err != nil {
		t.Fatal("unexpected error finding storage: ", err)
	}
	if sm.AvailableStorage() != 100 {
		t.Fatal("wrong amount of available storage")
	}
	_, err = sm.FindStorage(1, 100)
	if err != nil {
		t.Fatal("unexpected error finding storage: ", err)
	}
	if sm.AvailableStorage() != 0 {
		t.Fatal("wrong amount of available storage")
	}
	sm.AddBlob(&storageBlob{
		ProviderId: "p1",
		Amount:     100,
	})
	if sm.AvailableStorage() != 100 {
		t.Fatal("wrong amount of available storage")
	}
}

func TestFindStorage_MergesBlobs(t *testing.T) {
	sm := newStorageManager([]*storageBlob{}, noOpUpdateFn, time.Minute, &realClock{})
	sm.AddBlob(&storageBlob{
		ProviderId: "p1",
		Amount:     100,
		ContractId: "c1",
	})
	sm.AddBlob(&storageBlob{
		ProviderId: "p2",
		Amount:     1,
		ContractId: "c2",
	})

	// Add a third blob under the same contract as the first.
	// This should be merged with the first.
	sm.AddBlob(&storageBlob{
		ProviderId: "p1",
		Amount:     100,
		ContractId: "c1",
	})

	// We should now be able to retrieve the merged blob.
	_, err := sm.FindStorage(1, 200)
	if err != nil {
		t.Fatal("unexpected error finding storage: ", err)
	}
}

func TestFindStorage_MultipleBlobs(t *testing.T) {
	sm := newStorageManager([]*storageBlob{}, noOpUpdateFn, time.Minute, &realClock{})
	blobSize := 10000
	for i := 0; i < 10; i++ {
		sm.AddBlob(&storageBlob{
			ProviderId: "p1",
			Amount:     int64(blobSize),
			ContractId: fmt.Sprintf("%d", i),
		})
	}
	for i := 0; i < 2; i++ {
		blobs, err := sm.FindStorage(5, int64(blobSize))
		if err != nil {
			t.Fatal("unable to find storage")
		}
		if len(blobs) != 5 {
			t.Fatal("returned wrong number of blobs")
		}
		for _, blob := range blobs {
			if blob.Amount != int64(blobSize) {
				t.Fatal("returned blob has wrong size")
			}
		}
	}
}

func TestFindStorage_FailureLeavesStorageUnchanged(t *testing.T) {
	sm := newStorageManager([]*storageBlob{}, noOpUpdateFn, time.Minute, &realClock{})
	blobSize := int64(100)
	for i := 0; i < 10; i++ {
		sm.AddBlob(&storageBlob{
			ProviderId: "p1",
			Amount:     blobSize,
			ContractId: fmt.Sprintf("%d", i),
		})
	}
	_, err := sm.FindStorage(11, blobSize)
	if err == nil {
		t.Fatal("found storage unexpectedly")
	}
	if sm.AvailableStorage() != blobSize*10 {
		t.Fatal("wrong amount of available storage")
	}
	blobs, err := sm.FindStorage(10, blobSize)
	if err != nil {
		t.Fatal("unable to find storage")
	}
	if len(blobs) != 10 {
		t.Fatal("returned wrong number of blobs")
	}
	for _, blob := range blobs {
		if blob.Amount != blobSize {
			t.Fatal("blob has incorrect size")
		}
	}
}

func TestFindStorage_SmallFragments(t *testing.T) {
	sm := newStorageManager([]*storageBlob{}, noOpUpdateFn, time.Minute, &realClock{})
	amt := int64(1000000)
	sm.AddBlob(&storageBlob{
		ProviderId: "p1",
		Amount:     amt,
		ContractId: "c1",
	})
	for i := 0; i < 4; i++ {
		_, err := sm.FindStorage(1, amt/4)
		if err != nil {
			t.Fatal("unable to find storage")
		}
	}
	_, err := sm.FindStorage(1, 1)
	if err == nil {
		t.Fatal("found storage unexpectedly")
	}
}

func TestFindStorage_BigBlob(t *testing.T) {
	sm := newStorageManager([]*storageBlob{}, noOpUpdateFn, time.Minute, &realClock{})
	maxBlobSize := 1000
	for i := 0; i < 10; i++ {
		sm.AddBlob(&storageBlob{
			ProviderId: "p1",
			Amount:     int64(rand.Intn(1000) + 1),
			ContractId: fmt.Sprintf("c%d", i),
		})
	}
	_, err := sm.FindStorage(1, int64(maxBlobSize+1))
	if err == nil {
		t.Fatal("found storage unexpectedly")
	}
}

func TestFindStorage_Fuzz(t *testing.T) {
	sm := newStorageManager([]*storageBlob{}, noOpUpdateFn, time.Minute, &realClock{})
	maxAmt := 100000
	maxBlobSize := maxAmt / 4
	minBlobSize := 5000
	numContracts := 8
	tot := 0

	// Store storage in chunks
	for tot < maxAmt {
		blobSize := rand.Intn(maxAmt - tot + 1)
		if blobSize < minBlobSize {
			blobSize = minBlobSize
		}
		if blobSize > maxBlobSize {
			blobSize = maxBlobSize
		}
		cid := rand.Intn(numContracts)
		sm.AddBlob(&storageBlob{
			ProviderId: "p1",
			Amount:     int64(blobSize),
			ContractId: fmt.Sprintf("%d", cid),
		})
		tot += blobSize
	}

	// Retrieve storage in chunks. Due to fragmentation, we may
	// not be able to retrieve the total amount placed in
	maxUnusableStorage := maxAmt / 10
	for tot > maxUnusableStorage {
		blobSize := rand.Intn(minBlobSize)
		if blobSize > tot {
			blobSize = tot
		}
		blobs, err := sm.FindStorage(1, int64(blobSize))
		if err != nil {
			t.Fatal("unable to find storage")
		}
		if len(blobs) != 1 || blobs[0].Amount != int64(blobSize) {
			t.Fatal("returned incorrect blob")
		}
		tot -= blobSize
	}

	if sm.AvailableStorage() != int64(tot) {
		t.Fatal("incorrect storage amount")
	}
}

func TestMarkProvidersOffline(t *testing.T) {
	mc := mockClock{time.Now()}
	sm := newStorageManager([]*storageBlob{}, noOpUpdateFn, time.Minute, &mc)
	for i := 0; i < 10; i++ {
		sm.AddBlob(&storageBlob{
			ProviderId: fmt.Sprintf("p%d", i),
			Amount: 1024,
			ContractId: fmt.Sprintf("c%d", i),
		})
	}
	sm.MarkProvidersOffline([]string{"p1", "p2", "p7"}, time.Now().Add(5 * time.Minute))
	_, err := sm.FindStorage(7, 1024)
	if err != nil {
		t.Fatal("failed to find storage")
	}
	_, err = sm.FindStorage(1, 1024)
	if err == nil {
		t.Fatal("found storage with offline provider")
	}

	// Advance the clock
	mc.nextTime = time.Now().Add(10 * time.Minute)

	_, err = sm.FindStorage(3, 1024)
	if err != nil {
		t.Fatal("unable to find storage after providers marked online")
	}
}

func TestMarkProvidersOffline_Duplicates(t *testing.T) {
	mc := mockClock{time.Now()}
	sm := newStorageManager([]*storageBlob{}, noOpUpdateFn, time.Minute, &mc)
	for i := 0; i < 10; i++ {
		sm.AddBlob(&storageBlob{
			ProviderId: fmt.Sprintf("p%d", i),
			Amount: 1024,
			ContractId: fmt.Sprintf("c%d", i),
		})
	}
	sm.MarkProvidersOffline([]string{"p1", "p2"}, time.Now().Add(5 * time.Minute))
	sm.MarkProvidersOffline([]string{"p2", "p3"}, time.Now().Add(5 * time.Minute))
	_, err := sm.FindStorage(7, 1024)
	if err != nil {
		t.Fatal("failed to find storage")
	}
	_, err = sm.FindStorage(1, 512)
	if err == nil {
		t.Fatal("found storage with offline provider")
	}
	mc.nextTime = time.Now().Add(time.Hour)
	_, err = sm.FindStorage(6, 512)
	if err != nil {
		t.Fatal("failed to find storage")
	}
}
