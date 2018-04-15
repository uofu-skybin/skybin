package renter

import (
	"errors"
	"math/rand"
	"sync"
	"time"
)

// Tracks the storage available for use by the renter,
// serving as a local (possibly inconsistent) cache
// for the storage metadata on the metaserver.
//
// Periodically updates the cache from the metaserver
// by calling the provided UpdateFn.
//
// Safe for use by multiple concurrent goroutines.
type storageManager struct {
	mu sync.Mutex

	// Free blobs for use in uploads. Note: each
	// of the renter's storage contracts has at most
	// one associated blob in this list.
	freelist []*storageBlob

	updateFn        func() ([]*storageBlob, error)
	updateFreq      time.Duration
	lastCacheUpdate time.Time
	clock           clock
}

// Interface used to check current time. Eases testing.
type clock interface {
	Now() time.Time
}

type realClock struct{}

func (c realClock) Now() time.Time {
	return time.Now()
}

func newStorageManager(
	blobs []*storageBlob,
	updateFn func() ([]*storageBlob, error),
	updateFreq time.Duration,
	clock clock) *storageManager {

	return &storageManager{
		freelist:     blobs,
		updateFn:     updateFn,
		updateFreq:   updateFreq,
		clock:        clock,
	}
}

// Returns the total amount of storage available to the renter,
// including storage which may be currently unusable because e.g.
// a provider is offline. Calling this always updates the storage cache.
func (sm *storageManager) AvailableStorage() int64 {
	sm.mu.Lock()
	sm.updateCache()
	amt := int64(0)
	for _, blob := range sm.freelist {
		amt += blob.Amount
	}
	sm.mu.Unlock()
	return amt
}

func (sm *storageManager) AddBlob(blob *storageBlob) {
	sm.AddBlobs([]*storageBlob{blob})
}

func (sm *storageManager) AddBlobs(blobs []*storageBlob) {
	sm.mu.Lock()
	for _, blob := range blobs {
		sm.addBlob(blob)
	}
	sm.mu.Unlock()
}

// Finds storage blobs for use in an upload.
func (sm *storageManager) FindStorage(nblobs int, blobSize int64) ([]*storageBlob, error) {
	return sm.FindStorageExclude(nblobs, blobSize, map[string]bool{})
}

// Finds storage blobs for use in an upload. Does not return blobs located
// with providers whose IDs are in the given set.
func (sm *storageManager) FindStorageExclude(nblobs int, blobSize int64, providers map[string]bool) ([]*storageBlob, error) {
	sm.mu.Lock()
	sm.maybeUpdateCache()
	blobs, err := sm.findStorage(nblobs, blobSize, providers)
	sm.mu.Unlock()
	return blobs, err
}

func (sm *storageManager) addBlob(blob *storageBlob) {
	for _, existingBlob := range sm.freelist {
		if existingBlob.ContractId == blob.ContractId {
			existingBlob.Amount += blob.Amount
			return
		}
	}
	sm.freelist = append(sm.freelist, blob)
}

type candidate struct {
	*storageBlob
	idx int // Index of the blob in the freelist
}

func (sm *storageManager) findCandidates(blobSize int64, excludedProviders map[string]bool) []candidate {
	if len(sm.freelist) == 0 {
		return nil
	}

	startIdx := rand.Intn(len(sm.freelist))
	curr := startIdx
	candidates := []candidate{}

	for curr-startIdx < len(sm.freelist) {
		idx := curr % len(sm.freelist)
		blob := sm.freelist[idx]
		if blob.Amount >= blobSize {
			_, isExcluded := excludedProviders[blob.ProviderId]
			if !isExcluded {
				candidates = append(candidates, candidate{
					storageBlob: blob,
					idx:         idx,
				})
			}
		}
		curr++
	}
	return candidates
}

func (sm *storageManager) findStorage(nblobs int, blobSize int64, excludedProviders map[string]bool) ([]*storageBlob, error) {
	candidates := sm.findCandidates(blobSize, excludedProviders)
	blobs := []*storageBlob{}

	for i := 0; len(blobs) < nblobs && len(candidates) > 0; {
		candidate := candidates[i]
		blob := &storageBlob{
			ProviderId: candidate.ProviderId,
			Amount:     blobSize,
			Addr:       candidate.Addr,
			ContractId: candidate.ContractId,
		}
		blobs = append(blobs, blob)

		candidate.Amount -= blob.Amount

		if candidate.Amount < blobSize {
			candidates = append(candidates[:i], candidates[i+1:]...)
		}
		if len(candidates) == 0 {
			break
		}
		i = (i + 1) % len(candidates)
	}
	if len(blobs) < nblobs {
		for _, blob := range blobs {
			sm.addBlob(blob)
		}
		return nil, errors.New("Cannot find enough storage.")
	}
	for i := len(sm.freelist) - 1; i >= 0; i-- {
		if sm.freelist[i].Amount < kMinBlobSize {
			sm.freelist = append(sm.freelist[:i], sm.freelist[i+1:]...)
		}
	}
	return blobs, nil
}

func (sm *storageManager) maybeUpdateCache() {
	if len(sm.freelist) == 0 {
		// If it looks like we don't have any storage,
		// always pull a fresh view from the metaserver just
		// to be sure.
		sm.updateCache()
		return
	}
	if sm.clock.Now().Sub(sm.lastCacheUpdate) > sm.updateFreq {
		sm.updateCache()
	}
}

func (sm *storageManager) updateCache() {
	blobs, err := sm.updateFn()
	if err != nil {
		// TODO: handle
		return
	}
	shuffleBlobs(blobs)

	// Note that we may be updating the freelist to include
	// blobs that are being used in ongoing uploads. This is
	// a weakness here, but ultimately not a serious issue
	// since the metaserver serves as the source of truth
	// on what blobs are available.
	sm.freelist = blobs
	sm.lastCacheUpdate = sm.clock.Now()
}

func shuffleBlobs(blobs []*storageBlob) {
	for i := len(blobs) - 1; i >= 0; i-- {
		j := rand.Intn(i + 1)
		blobs[i], blobs[j] = blobs[j], blobs[i]
	}
}
