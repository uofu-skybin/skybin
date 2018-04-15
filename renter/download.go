package renter

import (
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path"
	"skybin/core"
	"skybin/provider"
	"strings"
	"time"

	"github.com/klauspost/reedsolomon"
)

type BlockDownloadStats struct {
	BlockId     string `json:"blockId"`
	ProviderId  string `json:"providerId"`
	Location    string `json:"location"`
	TotalTimeMs int64  `json:"totalTimeMs"`
	Error       string `json:"error,omitempty"`
}

type FileDownloadStats struct {
	FileId      string                `json:"fileId"`
	Name        string                `json:"name"`
	IsDir       bool                  `json:"isDir"`
	VersionNum  int                   `json:"versionNum"`
	DestPath    string                `json:"destPath"`
	TotalTimeMs int64                 `json:"totalTimeMs"`
	Blocks      []*BlockDownloadStats `json:"blocks"`
}

type DownloadStats struct {
	TotalTimeMs int64                `json:"totalTimeMs"`
	Files       []*FileDownloadStats `json:"files"`
}

type fileDownload struct {
	file     *core.File
	version  *core.Version
	destPath string

	// Decrypted encryption key and IV to decrypt the file
	aesKey []byte
	aesIV  []byte

	// Channel to notify the file's main download thread that
	// a block has finished downloading.
	blockCh chan *blockDownload

	// Number of blocks successfully/unsuccessfully downloaded.
	successfulBlocks int
	failedBlocks     int
	blockDownloads   []*blockDownload
	stats            *FileDownloadStats

	// Closed when the download is complete
	doneCh chan struct{}
	err    error
}

func newFileDownload(file *core.File, version *core.Version, destPath string,
	aesKey []byte, aesIV []byte) *fileDownload {
	fd := &fileDownload{
		file:     file,
		version:  version,
		destPath: destPath,
		aesKey:   aesKey,
		aesIV:    aesIV,
		stats: &FileDownloadStats{
			FileId:     file.ID,
			Name:       file.Name,
			IsDir:      file.IsDir,
			VersionNum: version.Num,
			DestPath:   destPath,
			Blocks:     []*BlockDownloadStats{},
		},
		blockCh: make(chan *blockDownload),
		doneCh:  make(chan struct{}),
	}
	return fd
}

func (fd *fileDownload) cleanup() {
	for _, blockDownload := range fd.blockDownloads {
		blockDownload.cleanup()
	}
}

type blockDownload struct {
	fileDownload *fileDownload
	block        *core.Block
	destFile     *os.File
	stats        *BlockDownloadStats
	err          error
}

func newBlockDownload(fileDownload *fileDownload, block *core.Block) (*blockDownload, error) {
	destFile, err := ioutil.TempFile("", "skybin_download")
	if err != nil {
		return nil, fmt.Errorf("Unable to create temp file to download block. Error: %s", err)
	}
	bd := &blockDownload{
		fileDownload: fileDownload,
		block:        block,
		destFile:     destFile,
		stats: &BlockDownloadStats{
			BlockId:    block.ID,
			ProviderId: block.Location.ProviderId,
			Location:   block.Location.Addr,
		},
	}
	return bd, nil
}

func (bd *blockDownload) cleanup() {
	bd.destFile.Close()
	os.Remove(bd.destFile.Name())
}

const (
	maxConcurrentDownloads = 8
	maxOpenFiles           = 256
)

func (r *Renter) Download(fileId string, destPath string, versionNum *int) (*DownloadStats, error) {
	file, err := r.GetFile(fileId)
	if err != nil {
		return nil, err
	}
	if file.IsDir && versionNum != nil {
		return nil, errors.New("Cannot give version option with folder download")
	}
	if len(destPath) == 0 {
		destPath, err = defaultDownloadLocation(file)
		if err != nil {
			return nil, err
		}
	}
	if file.IsDir {
		return r.downloadDir(file, destPath)
	}
	if len(file.Versions) == 0 {
		panic("Download: File has no versions")
	}

	// Download the latest version by default
	version := &file.Versions[len(file.Versions)-1]
	if versionNum != nil {
		version = findVersion(file, *versionNum)
		if version == nil {
			return nil, fmt.Errorf("Cannot find version %d", *versionNum)
		}
	}
	return r.downloadFile(file, version, destPath)
}

// Downloads a folder tree, including all subfolders and files.
// This may partially succeed, in that some children of the folder may
// be downloaded while others may fail.
func (r *Renter) downloadDir(dir *core.File, destPath string) (*DownloadStats, error) {
	startTime := time.Now()
	allFileStats, err := r.performDirDownload(dir, destPath)
	if err != nil {
		return nil, err
	}
	endTime := time.Now()
	totalTimeMs := toMilliseconds(endTime.Sub(startTime))
	return &DownloadStats{
		TotalTimeMs: totalTimeMs,
		Files:       allFileStats,
	}, nil
}

// Downloads a single version of a single file.
func (r *Renter) downloadFile(file *core.File, version *core.Version, destPath string) (*DownloadStats, error) {
	aesKey, aesIV, err := r.decryptEncryptionKeys(file)
	if err != nil {
		return nil, err
	}
	download := newFileDownload(file, version, destPath, aesKey, aesIV)
	r.downloadQ <- []*fileDownload{download}
	<-download.doneCh
	if download.err != nil {
		return nil, download.err
	}
	return &DownloadStats{
		TotalTimeMs: download.stats.TotalTimeMs,
		Files:       []*FileDownloadStats{download.stats},
	}, nil
}

func (r *Renter) performDirDownload(dir *core.File, destPath string) ([]*FileDownloadStats, error) {
	allFileStats := []*FileDownloadStats{}
	fileDownloads := []*fileDownload{}

	dirStats, err := mkdir(dir, destPath)
	if err != nil {
		return nil, err
	}
	allFileStats = append(allFileStats, dirStats)

	for _, child := range r.findChildren(dir) {
		relPath := strings.TrimPrefix(child.Name, dir.Name+"/")
		fullPath := path.Join(destPath, relPath)
		if child.IsDir {
			dirStats, err = mkdir(child, fullPath)
			if err != nil {
				return nil, fmt.Errorf("Unable to create folder %s. Error: %s", fullPath, err)
			}
			allFileStats = append(allFileStats, dirStats)
		} else {
			version := &child.Versions[len(child.Versions)-1]
			aesKey, aesIV, err := r.decryptEncryptionKeys(child)
			if err != nil {
				return nil, fmt.Errorf("Unable to decrypt encryption keys for file %s\n", child.Name)
			}
			fd := newFileDownload(child, version, fullPath, aesKey, aesIV)
			fileDownloads = append(fileDownloads, fd)
			allFileStats = append(allFileStats, fd.stats)
		}
	}
	r.downloadQ <- fileDownloads
	for _, download := range fileDownloads {
		<-download.doneCh
	}
	for _, download := range fileDownloads {
		if download.err != nil {
			return nil, err
		}
	}
	return allFileStats, nil
}

// Main thread for downloads. Dequeues and performs batches of downloads,
// maintaining limits on concurrency and the number of temporary files
// that can be open at any time. Funnelling downloads into this thread
// allows us to respect these limits across multiple independent download
// requests received by the server.
func (r *Renter) downloadThread() {
	blockQ := make(chan *blockDownload, 32)
	activeDownloads := 0
	activeBlocks := 0
	finishedDownloads := make(chan *fileDownload)

	go blockScheduleThread(blockQ)

	batch, ok := <-r.downloadQ
	if !ok {
		close(blockQ)
		return
	}
L:
	for {

		for _, download := range batch {
			for activeDownloads >= maxConcurrentDownloads ||
				download.version.NumDataBlocks+activeBlocks > maxOpenFiles {
				finishedDownload := <-finishedDownloads
				activeBlocks -= finishedDownload.version.NumDataBlocks
				activeDownloads--
			}

			go func(download *fileDownload) {
				r.performDownload(download, blockQ)
				close(download.doneCh)
				finishedDownloads <- download
			}(download)

			activeBlocks += download.version.NumDataBlocks
			activeDownloads++
		}

		select {

		// If the next batch of downloads is already queued,
		// leave the block schedule thread running to reuse
		// network connections with providers.
		case batch, ok = <-r.downloadQ:
			if !ok {
				break L
			}
		default:

			// If it's not, wait for all remaining downloads to finish.
			for activeDownloads > 0 {
				finishedDownload := <-finishedDownloads
				activeBlocks -= finishedDownload.version.NumDataBlocks
				activeDownloads--
			}

			// ... and restart the block schedule thread to close network
			// connections with all providers.
			close(blockQ)
			blockQ = make(chan *blockDownload, 32)
			go blockScheduleThread(blockQ)

			// Then wait for the next batch.
			batch, ok = <-r.downloadQ
			if !ok {
				break L
			}
		}
	}
	for activeDownloads > 0 {
		<-finishedDownloads
		activeDownloads--
	}
	close(blockQ)
}

// Blocks are downloaded in a dedicated thread determined
// by their location (i.e. the provider at which they're stored).
// This allows network connections with the provider to be shared
// across block downloads.
func blockScheduleThread(blockQ chan *blockDownload) {
	blockQueues := map[string]chan *blockDownload{}
	for download := range blockQ {
		providerID := download.block.Location.ProviderId
		q, exists := blockQueues[providerID]
		if !exists {
			q = make(chan *blockDownload, 16)
			blockQueues[providerID] = q
			go downloadWorker(download.block.Location.Addr, q)
		}
		q <- download
	}
	for _, q := range blockQueues {
		close(q)
	}
}

func downloadWorker(providerAddr string, inputQueue chan *blockDownload) {
	client := provider.NewClient(providerAddr, &http.Client{})
	for download := range inputQueue {
		if download.block.Location.Addr != providerAddr {
			panic("downloadWorker: Received block to download from incorrect provider")
		}

		startTime := time.Now()
		ownerID := download.fileDownload.file.OwnerID
		err := downloadBlock(client, ownerID, download.block, download.destFile)
		if err != nil {
			download.err = err
			download.stats.Error = err.Error()
		}
		endTime := time.Now()
		totalTimeMs := toMilliseconds(endTime.Sub(startTime))
		download.stats.TotalTimeMs = totalTimeMs

		// Let the thread running the associated file download know that
		// this block has been downloaded.
		//
		// This cannot block. If it does, we risk deadlocking
		// as the download thread waits for this thread to start the next block.
		go func(download *blockDownload) { download.fileDownload.blockCh <- download }(download)
	}
}

// Downloads a block and checks that its hash is correct.
func downloadBlock(client *provider.Client, ownerID string, block *core.Block, destFile *os.File) error {
	blockReader, err := client.GetBlock(ownerID, block.ID)
	if err != nil {
		return err
	}
	defer blockReader.Close()
	h := sha256.New()
	mw := io.MultiWriter(destFile, h)
	n, err := io.Copy(mw, blockReader)
	if err != nil {
		return fmt.Errorf("Cannot write block to local file. Error: %s", err)
	}
	if n != block.Size {
		return errBlockCorrupted
	}
	blockHash := base64.URLEncoding.EncodeToString(h.Sum(nil))
	if blockHash != block.Sha256Hash {
		return errBlockCorrupted
	}
	return nil
}

// Performs a download, sending individual file blocks to blockQ to be downloaded by a dedicated thread.
// IMPORTANT: This takes responsibility for cleaning up any temp files created during the download.
func (r *Renter) performDownload(download *fileDownload, blockQ chan *blockDownload) {
	startTime := time.Now()
	r.doPerformDownload(download, blockQ)
	endTime := time.Now()
	totalTimeMs := toMilliseconds(endTime.Sub(startTime))
	download.stats.TotalTimeMs = totalTimeMs

	if download.failedBlocks == 0 {
		download.cleanup()
		return
	}

	// Blocks that were corrupted but recovered via erasure coding
	// need to be submitted to the restore Q to be uploaded to new
	// providers.
	// TODO: This only recovers data blocks. recover parity blocks as well
	restoreNeeded := false
	for idx, blockDownload := range download.blockDownloads {
		if idx < download.version.NumDataBlocks && blockDownload.err == errBlockCorrupted {
			restoreNeeded = true
			break
		}
	}
	if !restoreNeeded {
		download.cleanup()
		return
	}
	batch := &recoveredBlockBatch{
		file:    *download.file,
		version: *download.version,
	}
	for idx, blockDownload := range download.blockDownloads {
		if idx < download.version.NumDataBlocks && blockDownload.err == errBlockCorrupted {
			rc := &recoveredBlock{
				block:    *blockDownload.block,
				contents: blockDownload.destFile,
			}
			batch.blocks = append(batch.blocks, rc)
		} else {
			blockDownload.cleanup()
		}
	}
	go func() { r.restoreQ <- batch }()
}

func (r *Renter) doPerformDownload(download *fileDownload, blockQ chan *blockDownload) {
	pendingBlocks := 0
	for i := 0; i < download.version.NumDataBlocks; i++ {
		bd, err := newBlockDownload(download, &download.version.Blocks[i])
		if err != nil {
			download.err = err
			return
		}
		download.blockDownloads = append(download.blockDownloads, bd)
		download.stats.Blocks = append(download.stats.Blocks, bd.stats)
		blockQ <- bd
		pendingBlocks++
	}

	for pendingBlocks > 0 {
		finishedBlock := <-download.blockCh
		pendingBlocks--
		if finishedBlock.err != nil {
			r.logger.Printf("Error downloading block %s for file %s: %s\n",
				finishedBlock.block.ID, download.file.ID, finishedBlock.err)

			download.failedBlocks++

			if download.err != nil {
				continue
			}

			// Can we make up for the lost block with a parity block?
			if download.failedBlocks > download.version.NumParityBlocks {
				download.err = fmt.Errorf("Unable to download enough blocks to reconstruct file %s.",
					download.file.Name)
				continue
			}

			// We can. Try downloading the next parity block.
			parityIdx := download.version.NumDataBlocks + download.failedBlocks - 1
			parityBlock := &download.version.Blocks[parityIdx]
			bd, err := newBlockDownload(download, parityBlock)
			if err != nil {
				download.err = err
				continue
			}
			download.blockDownloads = append(download.blockDownloads, bd)
			download.stats.Blocks = append(download.stats.Blocks, bd.stats)
			blockQ <- bd
			pendingBlocks++
			continue
		}

		download.successfulBlocks++
	}
	if download.err != nil {
		return
	}

	err := reconstructFile(download)
	if err != nil {
		download.err = err
		return
	}
}

// Completes a download by reconstructing the file from the downloaded
// blocks and placing it at the destination path.
func reconstructFile(download *fileDownload) error {
	var blockFiles []*os.File
	for _, blockDownload := range download.blockDownloads {
		blockFiles = append(blockFiles, blockDownload.destFile)
	}

	needsReconstruction := download.failedBlocks > 0
	if needsReconstruction {
		for idx, blockFile := range blockFiles {
			_, err := blockFile.Seek(0, os.SEEK_SET)
			if err != nil {
				return fmt.Errorf("Unable to seek block file. Error: %s", err)
			}
			if download.blockDownloads[idx].err != nil {
				err := blockFile.Truncate(0)
				if err != nil {
					return err
				}
			}
		}

		// Build the lists of valid and invalid blocks to be reconstructed.
		var validBlocks []io.Reader
		var blocksToReconstruct []io.Writer
		for idx, file := range blockFiles {
			var validBlock io.Reader = nil
			var blockToReconstruct io.Writer = nil

			if download.blockDownloads[idx].err == nil {
				validBlock = file
			} else if idx < download.version.NumDataBlocks {
				blockToReconstruct = file
			}

			validBlocks = append(validBlocks, validBlock)
			blocksToReconstruct = append(blocksToReconstruct, blockToReconstruct)
		}
		for len(validBlocks) < download.version.NumDataBlocks+download.version.NumParityBlocks {
			validBlocks = append(validBlocks, nil)
			blocksToReconstruct = append(blocksToReconstruct, nil)
		}

		// Reconstruct the missing blocks.
		decoder, err := reedsolomon.NewStream(download.version.NumDataBlocks,
			download.version.NumParityBlocks)
		if err != nil {
			return fmt.Errorf("Unable to construct decoder. Error: %s", err)
		}
		err = decoder.Reconstruct(validBlocks, blocksToReconstruct)
		if err != nil {
			return fmt.Errorf("Failed to reconstruct file. Error: %s", err)
		}
		blockFiles = blockFiles[:download.version.NumDataBlocks]
	}
	if len(blockFiles) != download.version.NumDataBlocks {
		panic("block files should contain file.NumDataBlocks files")
	}

	for _, f := range blockFiles {
		_, err := f.Seek(0, os.SEEK_SET)
		if err != nil {
			return fmt.Errorf("Unable to seek block file. Error: %s", err)
		}
	}

	// Remove padding of the last block
	if download.version.PaddingBytes > 0 {
		f := blockFiles[len(blockFiles)-1]
		st, err := f.Stat()
		if err != nil {
			return fmt.Errorf("Unable to stat block file. Error: %s", err)
		}
		err = f.Truncate(st.Size() - download.version.PaddingBytes)
		if err != nil {
			return fmt.Errorf("Unable to truncate padding bytes. Error: %s", err)
		}
	}

	// Decrypt
	aesCipher, err := aes.NewCipher(download.aesKey)
	if err != nil {
		return fmt.Errorf("Unable to create aes cipher. Error: %v", err)
	}
	streamReader := cipher.StreamReader{
		S: cipher.NewCFBDecrypter(aesCipher, download.aesIV),
		R: io.MultiReader(convertToReaderSlice(blockFiles)...),
	}
	temp2, err := ioutil.TempFile("", "skybin_download")
	if err != nil {
		return fmt.Errorf("Unable to create temp file to decrypt download. Error: %v", err)
	}
	defer temp2.Close()
	defer os.Remove(temp2.Name())
	_, err = io.Copy(temp2, streamReader)
	if err != nil {
		return fmt.Errorf("Unable to decrypt file. Error: %s", err)
	}
	_, err = temp2.Seek(0, os.SEEK_SET)
	if err != nil {
		return fmt.Errorf("Unable to seek to beginning of decrypted temp. Error: %s", err)
	}

	// Decompress
	zr, err := zlib.NewReader(temp2)
	if err != nil {
		return fmt.Errorf("Unable to initialize decompression reader. Error: %v", err)
	}
	defer zr.Close()
	outFile, err := os.Create(download.destPath)
	if err != nil {
		return fmt.Errorf("Unable to create destination file. Error: %v", err)
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, zr)
	if err != nil {
		return fmt.Errorf("Unable to decompress file. Error: %v", err)
	}
	return nil
}

func defaultDownloadLocation(f *core.File) (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	destPath := path.Join(user.HomeDir, path.Base(f.Name))
	if _, err := os.Stat(destPath); err == nil {
		for i := 1; ; i++ {
			d := fmt.Sprintf("%s (%d)", destPath, i)
			if _, err := os.Stat(d); os.IsNotExist(err) {
				return d, nil
			}
		}
	}
	return destPath, nil
}

// Helper to create a directory and the associated file download stats.
func mkdir(dir *core.File, destPath string) (*FileDownloadStats, error) {
	err := os.MkdirAll(destPath, 0777)
	if err != nil {
		return nil, err
	}
	return &FileDownloadStats{
		FileId:   dir.ID,
		Name:     dir.Name,
		IsDir:    true,
		DestPath: destPath,
		Blocks:   []*BlockDownloadStats{},
	}, nil
}

func convertToWriterSlice(files []*os.File) []io.Writer {
	var res []io.Writer
	for _, f := range files {
		if f == nil {
			// Must explicitly append nil since Go will otherwise
			// not treat f as nil in subsequent equality checks
			res = append(res, nil)
		} else {
			res = append(res, f)
		}

	}
	return res
}

func convertToReaderSlice(files []*os.File) []io.Reader {
	var res []io.Reader
	for _, f := range files {
		if f == nil {
			res = append(res, nil)
		} else {
			res = append(res, f)
		}
	}
	return res
}

func findVersion(file *core.File, versionNum int) *core.Version {
	for i := 0; i < len(file.Versions); i++ {
		if file.Versions[i].Num == versionNum {
			return &file.Versions[i]
		}
	}
	return nil
}

func toMilliseconds(d time.Duration) int64 {
	return int64(d.Seconds() * 1000.0)
}
