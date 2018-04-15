package renter

import (
	"os"
	"skybin/core"
	"skybin/provider"
	"net/http"
	"io"
)

// Information about corrupted blocks recovered through erasure
// coding as part of a single file download, including the contents
// of the recovered blocks.
type recoveredBlockBatch struct {
	// A copy of the SkyBin file at the time of the download
	file core.File
	// A copy of the version that was being downloaded
	version core.Version
	// The blocks that were corrupted but whose contents
	// were recovered through erasure coding.
	blocks []*recoveredBlock
}

func (batch *recoveredBlockBatch) cleanup() {
	for _, rb := range batch.blocks {
		rb.cleanup()
	}
}

// A single block recovered via erasure coding during a download
// after being retrieved in a corrupted state from the provider.
// where it was stored.
type recoveredBlock struct {
	// A copy of the block metadata for the recovered block
	block    core.Block
	// The valid block contents recovered via erasure coding
	contents *os.File
}

func (rb *recoveredBlock) cleanup() {
	rb.contents.Close()
	os.Remove(rb.contents.Name())
}

// Takes recovered block batches from the restore queue and
// re-uploads the corrupted blocks to new providers, updating
// file metadata appropriately. Note that this thread takes
// ownership for cleaning up recovered blocks (i.e. closing and removing
// the temp files containing their contents) as soon as they
// are submitted to the restore queue.
func (r *Renter) blockRestoreThread() {
	r.logger.Println("starting block recovery thread")
	for batch := range r.restoreQ {
		r.restoreBlockBatch(batch)
		batch.cleanup()
	}
	r.logger.Println("block recovery thread shutting down")
}

func (r *Renter) restoreBlockBatch(batch *recoveredBlockBatch) {
	if len(batch.blocks) == 0 {
		r.logger.Println("block recovery thread: given batch to restore with no blocks")
		return
	}
	r.logger.Printf("block recovery thread: attempting to restore corrupted blocks for file %s\n", batch.file.Name)

	// First, determine which blocks in the batch actually
	// need to be uploaded to a new provider. Some recovered
	// blocks may be "stale" if two or more downloads of the same file
	// submit a batch and this thread has already recovered the first batch.
	file, err := r.GetFile(batch.file.ID)
	if err != nil {
		// The file no longer exists. We don't care about
		// recovering its corrupted blocks.
		return
	}
	currVersion := findVersion(file, batch.version.Num)
	if currVersion == nil {
		// The downloaded version no longer exists. Ditto.
		return
	}

	// It may be that we've previously recovered some blocks in
	// the batch but not all of them, so we next filter them to determine
	// which blocks actually need to be re-uploaded.
	badBlocks := []*recoveredBlock{}
	for _, rb := range batch.blocks {
		if rb.block.Num >= len(currVersion.Blocks) {
			continue
		}
		currBlock := &currVersion.Blocks[rb.block.Num]
		if currBlock.ID != rb.block.ID {
			// We've already recovered this one.
			continue
		}
		badBlocks = append(badBlocks, rb)
	}
	if len(badBlocks) == 0 {
		return
	}

	// When we upload the corrupted blocks, we don't want them to be
	// stored on providers who have corrupted blocks.
	badProviders := map[string]bool{}
	for _, cb := range batch.blocks {
		badProviders[cb.block.Location.ProviderId] = true
	}

	// Now upload the bad blocks to new providers and create new
	// metadata for the file version.
	newVersion := *currVersion
	blockSize := badBlocks[0].block.Size
	blobsToReturn := []*storageBlob{}
	for len(badBlocks) > 0 {
		blobs, err := r.storageManager.FindStorageExclude(len(badBlocks), blockSize, badProviders)
		if err != nil {
			break
		}
		failures := []int{}
		for idx := 0; idx < len(badBlocks); idx++ {
			blob := blobs[idx]
			badBlock := badBlocks[idx]
			contents := io.NewSectionReader(badBlock.contents, 0, blockSize)

			client := provider.NewClient(blob.Addr, &http.Client{})
			err := client.AuthorizeRenter(r.privKey, r.Config.RenterId)
			if err != nil {
				failures = append(failures, idx)
				continue
			}
			err = client.PutBlock(r.Config.RenterId, badBlock.block.ID, contents, blockSize)
			if err != nil {
				failures = append(failures, idx)
				continue
			}
			oldBlock := badBlock.block
			newBlock := oldBlock
			newBlock.Location = core.BlockLocation{
				ProviderId: blob.ProviderId,
				Addr: blob.Addr,
				ContractId: blob.ContractId,
			}
			newVersion.Blocks[newBlock.Num] = newBlock
		}
		stillBadBlocks := []*recoveredBlock{}
		for idx := range failures {
			stillBadBlocks = append(stillBadBlocks, badBlocks[idx])
			blobsToReturn = append(blobsToReturn, blobs[idx])
			badProviders[blobs[idx].ProviderId] = true
		}
		badBlocks = stillBadBlocks
	}
	if len(blobsToReturn) > 0 {
		r.storageManager.AddBlobs(blobsToReturn)
	}
	if len(badBlocks) > 0 {

		// We didn't manage to restore all of the recovered blocks.
		// TODO: The smart thing to do here would be to cache their contents
		// locally to ensure we don't lose them and then attempt to recover
		// them later.
		r.logger.Printf("block recovery thread: failed to restore all corrupted blocks for file %s\n",
			batch.file.Name)
		failedIDs := []string{}
		for _, badBlock := range badBlocks {
			failedIDs = append(failedIDs, badBlock.block.ID)
		}
		r.logger.Printf("block recovery thread: failed to restore blocks %+v\n", failedIDs)
	}

	// Regardless of whether we did or didn't retore all of the blocks,
	// update the file version to reflect any blocks we did recover.
	// TODO: There is a race here if the file version was deleted or
	// otherwise changed between GetFile() and this call.
	err = r.updateFileVersion(batch.file.ID, batch.version.Num, &newVersion)
	if err != nil {
		r.logger.Printf("block recovery thread: error updating file version %d for file %s: %s\n",
			batch.version.Num, batch.file.Name, err)
	}
}
