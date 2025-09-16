package consensus

import (
	"august/blockchain"
	"encoding/base64"
	"fmt"
)

// BlockRequestFunc is a function that can request blocks from peers
type BlockRequestFunc func(blockHashes []string) ([]*blockchain.Block, error)

// LogFunc is a function for logging messages
type LogFunc func(format string, args ...interface{})

// ChainDownloader handles downloading candidate chains
type ChainDownloader struct {
	manager       *CandidateManager
	requestBlocks BlockRequestFunc
	logf          LogFunc
}

// NewChainDownloader creates a new chain downloader
func NewChainDownloader(manager *CandidateManager, requestBlocks BlockRequestFunc, logf LogFunc) *ChainDownloader {
	return &ChainDownloader{
		manager:       manager,
		requestBlocks: requestBlocks,
		logf:          logf,
	}
}

// DownloadCandidateChain downloads all blocks for a candidate chain
func (cd *ChainDownloader) DownloadCandidateChain(candidate *CandidateChain) error {
	defer func() {
		// Mark as complete or failed
		if candidate.downloadStatus.Load() == 0 { // still downloading
			candidate.downloadStatus.Store(1) // mark complete
		}
	}()

	cd.logf("Starting block download for candidate %s", candidate.ID)

	// Download blocks in batches
	batchSize := uint64(100)
	startHeight := uint64(1)
	endHeight := candidate.expectedHeight.Load()

	for startHeight <= endHeight {
		count := batchSize
		if startHeight+count > endHeight+1 {
			count = endHeight - startHeight + 1
		}

		// Check if we should abort (better candidate appeared)
		if cd.manager.ShouldAbortDownload(candidate) {
			cd.logf("Aborting download for candidate %s - better option found", candidate.ID)
			candidate.downloadStatus.Store(2) // mark failed
			return fmt.Errorf("download aborted for better candidate")
		}

		// Convert headers to hashes for this batch
		var blockHashes []string
		for _, header := range candidate.Headers {
			if header.Height >= startHeight && header.Height < startHeight+count {
				hash := header.GetHash()
				hashStr := base64.StdEncoding.EncodeToString(hash[:])
				blockHashes = append(blockHashes, hashStr)
			}
		}

		cd.logf("Candidate %s: downloading %d blocks %d-%d from multiple peers", candidate.ID, len(blockHashes), startHeight, startHeight+count-1)

		blocks, err := cd.requestBlocks(blockHashes)
		if err != nil {
			cd.logf("Failed to download blocks for candidate %s: %v", candidate.ID, err)
			candidate.downloadStatus.Store(2) // mark failed
			return fmt.Errorf("failed to download blocks: %w", err)
		}

		cd.logf("Candidate %s: downloaded %d blocks from peers", candidate.ID, len(blocks))

		// Add blocks to candidate's isolated chain store
		for _, block := range blocks {
			if err := candidate.ChainStore.AddBlock(block); err != nil {
				cd.logf("Failed to add block to candidate %s: %v", candidate.ID, err)
				candidate.downloadStatus.Store(2) // mark failed
				return fmt.Errorf("failed to add block to candidate: %w", err)
			}
			candidate.currentHeight.Store(block.Header.Height)
		}

		cd.logf("Candidate %s: added %d blocks, now at height %d",
			candidate.ID, len(blocks), candidate.currentHeight.Load())

		startHeight += count
	}

	cd.logf("Candidate %s: download complete, evaluating for promotion", candidate.ID)

	// Download complete - evaluate for promotion
	if err := cd.manager.EvaluateCandidateForPromotion(candidate); err != nil {
		cd.logf("Candidate %s not promoted: %v", candidate.ID, err)
		return err
	}

	cd.logf("Successfully promoted candidate %s to active chain", candidate.ID)
	return nil
}
