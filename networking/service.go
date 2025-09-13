package networking

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"august/blockchain"
	store "august/storage"
)

// RelayBlock broadcasts a block to all connected peers, optionally excluding a specific peer
// Returns a completion channel that will be closed when the relay operation completes
func RelayBlock(server *Server, block *blockchain.Block, excludePeerAddr string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		blockHash := blockchain.HashBlockHeader(&block.Header)

		// Add to recent blocks to prevent duplication when we receive it back from peers
		// (only when not excluding anyone, meaning this is a local block)
		if excludePeerAddr == "" {
			server.recentBlocksMu.Lock()
			server.recentBlocks[blockHash] = time.Now()
			server.recentBlocksMu.Unlock()
		}

		server.logf("Relaying block %x to other peers (excluding %s)", blockHash[:8], excludePeerAddr)

		// Create the block payload
		blockPayload := NewBlockPayload{Block: block}
		msg, err := NewMessage(MessageTypeNewBlock, blockPayload)
		if err != nil {
			server.logf("Failed to create relay message for block %x: %v", blockHash[:8], err)
			return
		}

		// Send to all connected peers except the sender
		relayCount := 0
		for _, peer := range server.peerManager.GetConnectedPeers() {
			if peer.Address != excludePeerAddr && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := server.reqRespClient.SendNotification(peer.Address, msg); err != nil {
					server.logf("Failed to relay block %x to peer %s: %v", blockHash[:8], peer.Address, err)
				} else {
					server.logf("Relayed block %x to peer %s", blockHash[:8], peer.Address)
					relayCount++
				}
			}
		}

		server.logf("Successfully relayed block %x to %d peers", blockHash[:8], relayCount)
	}()

	return complete
}

// RelayBlockHeader broadcasts a block header to all connected peers (headers-first), optionally excluding a specific peer
// Returns a completion channel that will be closed when the relay operation completes
func RelayBlockHeader(server *Server, header *blockchain.BlockHeader, excludePeerAddr string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		blockHash := blockchain.HashBlockHeader(header)

		// Add to recent blocks to prevent duplication when we receive it back from peers
		// (only when not excluding anyone, meaning this is a local header)
		if excludePeerAddr == "" {
			server.recentBlocksMu.Lock()
			server.recentBlocks[blockHash] = time.Now()
			server.recentBlocksMu.Unlock()
		}

		server.logf("Relaying block header %x (height %d) to other peers (excluding %s)",
			blockHash[:8], header.Height, excludePeerAddr)

		// Create the header payload
		headerPayload := NewBlockHeaderPayload{Header: *header}
		msg, err := NewMessage(MessageTypeNewBlockHeader, headerPayload)
		if err != nil {
			server.logf("Failed to create relay message for header %x: %v", blockHash[:8], err)
			return
		}

		// Send to all connected peers except the sender
		relayCount := 0
		for _, peer := range server.peerManager.GetConnectedPeers() {
			if peer.Address != excludePeerAddr && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := server.reqRespClient.SendNotification(peer.Address, msg); err != nil {
					server.logf("Failed to relay header %x to peer %s: %v", blockHash[:8], peer.Address, err)
				} else {
					server.logf("Relayed header %x to peer %s", blockHash[:8], peer.Address)
					relayCount++
				}
			}
		}

		server.logf("Successfully relayed header %x to %d peers", blockHash[:8], relayCount)
	}()

	return complete
}

// BroadcastTransaction broadcasts a transaction to all connected peers
// Returns a completion channel that will be closed when the broadcast operation completes
func BroadcastTransaction(server *Server, tx *blockchain.Transaction) <-chan struct{} {
	return broadcastTransactionToAllExcept(server, tx, "")
}

// broadcastTransactionToAllExcept broadcasts a transaction to all connected peers except the specified one
// Returns a completion channel that will be closed when the broadcast operation completes
func broadcastTransactionToAllExcept(server *Server, tx *blockchain.Transaction, excludePeerAddr string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		// Create the transaction payload
		txPayload := NewTxPayload{Transaction: tx}
		msg, err := NewMessage(MessageTypeNewTx, txPayload)
		if err != nil {
			server.logf("Failed to create transaction message: %v", err)
			return
		}

		// Log what we're doing
		if excludePeerAddr == "" {
			server.logf("Broadcasting transaction to all peers")
		} else {
			server.logf("Relaying transaction to other peers (excluding %s)", excludePeerAddr)
		}

		// Send to all connected peers
		sentCount := 0
		for _, peer := range server.peerManager.GetConnectedPeers() {
			if peer.Address != excludePeerAddr && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := server.reqRespClient.SendNotification(peer.Address, msg); err != nil {
					server.logf("Failed to send transaction to peer %s: %v", peer.Address, err)
				} else {
					server.logf("Sent transaction to peer %s", peer.Address)
					sentCount++
				}
			}
		}

		server.logf("Successfully sent transaction to %d peers", sentCount)
	}()

	return complete
}

// RequestBlockFromPeer requests a block and waits for the response
func RequestBlockFromPeer(server *Server, peerAddress string, blockHash string) (*blockchain.Block, error) {
	requestPayload := RequestBlockPayload{BlockHash: blockHash}

	msg, err := NewMessage(MessageTypeRequestBlock, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request message: %w", err)
	}

	response, err := server.reqRespClient.SendRequest(peerAddress, msg)
	if err != nil {
		return nil, err
	}

	// Cast back to Message and parse the response as a NewBlock message
	responseMsg := response.(*Message)
	if responseMsg.Type != MessageTypeNewBlock {
		return nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type)
	}

	var blockPayload NewBlockPayload
	if err := responseMsg.ParsePayload(&blockPayload); err != nil {
		return nil, fmt.Errorf("failed to parse block response: %w", err)
	}

	server.logf("Received block %s from %s", blockHash, peerAddress)
	return blockPayload.Block, nil
}

// RequestPeersFromPeer requests peers and waits for the response
func RequestPeersFromPeer(server *Server, peerAddress string, maxPeers int) ([]string, error) {
	requestPayload := RequestPeersPayload{MaxPeers: maxPeers}

	msg, err := NewMessage(MessageTypeRequestPeers, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request message: %w", err)
	}

	response, err := server.reqRespClient.SendRequest(peerAddress, msg)
	if err != nil {
		return nil, err
	}

	// Cast back to Message and parse the response as a SharePeers message
	responseMsg := response.(*Message)
	if responseMsg.Type != MessageTypeSharePeers {
		return nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type)
	}

	var sharePayload SharePeersPayload
	if err := responseMsg.ParsePayload(&sharePayload); err != nil {
		return nil, fmt.Errorf("failed to parse peers response: %w", err)
	}

	server.logf("Received %d peers from %s", len(sharePayload.Peers), peerAddress)
	return sharePayload.Peers, nil
}

// RequestChainHeadFromPeer requests the current chain head from a peer
func RequestChainHeadFromPeer(server *Server, peerAddress string) (*ChainHeadPayload, error) {
	requestPayload := RequestChainHeadPayload{}

	msg, err := NewMessage(MessageTypeRequestChainHead, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain head request: %w", err)
	}

	response, err := server.reqRespClient.SendRequest(peerAddress, msg)
	if err != nil {
		return nil, err
	}

	responseMsg := response.(*Message)
	if responseMsg.Type != MessageTypeChainHead {
		return nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type)
	}

	var headPayload ChainHeadPayload
	if err := responseMsg.ParsePayload(&headPayload); err != nil {
		return nil, fmt.Errorf("failed to parse chain head response: %w", err)
	}

	server.logf("Received chain head from %s: height=%d", peerAddress, headPayload.Height)
	return &headPayload, nil
}

// RequestHeadersFromPeer requests block headers in a range from a peer
func RequestHeadersFromPeer(server *Server, peerAddress string, startHeight uint64, count uint64) ([]blockchain.BlockHeader, error) {
	requestPayload := RequestHeadersPayload{
		StartHeight: startHeight,
		Count:       count,
	}

	msg, err := NewMessage(MessageTypeRequestHeaders, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create headers request: %w", err)
	}

	response, err := server.reqRespClient.SendRequest(peerAddress, msg)
	if err != nil {
		return nil, err
	}

	responseMsg := response.(*Message)
	if responseMsg.Type != MessageTypeHeaders {
		return nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type)
	}

	var headersPayload HeadersPayload
	if err := responseMsg.ParsePayload(&headersPayload); err != nil {
		return nil, fmt.Errorf("failed to parse headers response: %w", err)
	}

	server.logf("Received %d headers from %s starting at height %d", len(headersPayload.Headers), peerAddress, startHeight)
	return headersPayload.Headers, nil
}

// RequestBlocksFromPeer requests full blocks in a range from a peer
func RequestBlocksFromPeer(server *Server, peerAddress string, startHeight uint64, count uint64) ([]*blockchain.Block, error) {
	requestPayload := RequestBlocksPayload{
		StartHeight: startHeight,
		Count:       count,
	}

	msg, err := NewMessage(MessageTypeRequestBlocks, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocks request: %w", err)
	}

	response, err := server.reqRespClient.SendRequest(peerAddress, msg)
	if err != nil {
		return nil, err
	}

	responseMsg := response.(*Message)
	if responseMsg.Type != MessageTypeBlocks {
		return nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type)
	}

	var blocksPayload BlocksPayload
	if err := responseMsg.ParsePayload(&blocksPayload); err != nil {
		return nil, fmt.Errorf("failed to parse blocks response: %w", err)
	}

	server.logf("Received %d blocks from %s starting at height %d", len(blocksPayload.Blocks), peerAddress, startHeight)
	return blocksPayload.Blocks, nil
}

// RequestBlocksByHashesFromPeer requests specific blocks by their hashes from a peer
func RequestBlocksByHashesFromPeer(server *Server, peerAddress string, blockHashes []string) ([]*blockchain.Block, error) {
	requestPayload := RequestBlocksPayload{
		Hashes: blockHashes,
	}

	msg, err := NewMessage(MessageTypeRequestBlocks, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocks request: %w", err)
	}

	response, err := server.reqRespClient.SendRequest(peerAddress, msg)
	if err != nil {
		return nil, err
	}

	responseMsg := response.(*Message)
	if responseMsg.Type != MessageTypeBlocks {
		return nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type)
	}

	var blocksPayload BlocksPayload
	if err := responseMsg.ParsePayload(&blocksPayload); err != nil {
		return nil, fmt.Errorf("failed to parse blocks response: %w", err)
	}

	server.logf("Received %d requested blocks from %s", len(blocksPayload.Blocks), peerAddress)
	return blocksPayload.Blocks, nil
}

// EvaluateChainHead performs headers-first evaluation of a peer's chain
func EvaluateChainHead(server *Server, peerAddress string, peerChainHead *ChainHeadPayload) error {
	// Get our current chain
	ourChain, err := server.config.Store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get our chain: %w", err)
	}

	ourHeight := uint64(0)
	ourWork := "0"
	if len(ourChain.Blocks) > 0 {
		ourHeight = ourChain.Blocks[len(ourChain.Blocks)-1].Header.Height
		ourWork = ourChain.Blocks[len(ourChain.Blocks)-1].Header.TotalWork
	}

	server.logf("Evaluating chain from %s: our height=%d work=%s, peer height=%d work=%s",
		peerAddress, ourHeight, ourWork, peerChainHead.Height, peerChainHead.TotalWork)

	// Quick work comparison - if peer is not better, skip everything
	if blockchain.CompareWork(peerChainHead.TotalWork, ourWork) <= 0 {
		server.logf("Peer %s chain not better than ours, skipping", peerAddress)
		return nil
	}

	// Phase 1: Download headers first (lightweight evaluation)
	server.logf("Peer chain looks better, downloading headers from %s", peerAddress)
	headers, err := RequestHeadersFromPeer(server, peerAddress, 1, peerChainHead.Height)
	if err != nil {
		return fmt.Errorf("failed to download headers: %w", err)
	}

	// Validate header chain before downloading any blocks
	if !ValidateHeaderChain(headers) {
		server.logf("Header chain from %s invalid, rejecting", peerAddress)
		return fmt.Errorf("invalid header chain from %s", peerAddress)
	}

	// Double-check work calculation from headers
	finalHeaderWork := headers[len(headers)-1].TotalWork
	if blockchain.CompareWork(finalHeaderWork, ourWork) <= 0 {
		server.logf("Header chain work not better after validation, skipping")
		return nil
	}

	// Phase 2: Headers look good, start candidate chain download
	server.logf("Headers validated and better, starting candidate chain download")
	return StartCandidateChainDownload(server, peerAddress, headers, peerChainHead)
}

// ValidateHeaderChain performs lightweight validation on a chain of headers
func ValidateHeaderChain(headers []blockchain.BlockHeader) bool {
	if len(headers) == 0 {
		return false
	}

	// Basic validation - each header should link to the previous
	for i := 1; i < len(headers); i++ {
		prevHash := blockchain.HashBlockHeader(&headers[i-1])
		if headers[i].PreviousHash != prevHash {
			return false
		}

		// Height should increment by 1
		if headers[i].Height != headers[i-1].Height+1 {
			return false
		}

		// TODO: Add PoW validation, difficulty checks, etc.
	}

	return true
}

// StartCandidateChainDownload creates a candidate chain and downloads blocks
func StartCandidateChainDownload(server *Server, peerAddress string, headers []blockchain.BlockHeader, chainHead *ChainHeadPayload) error {
	// Generate candidate ID
	candidateID := fmt.Sprintf("%s-%d-%s", peerAddress, chainHead.Height, chainHead.HeadHash[:16])

	// Create candidate chain with isolated storage
	candidateStore := store.NewMemoryChainStore()

	candidate := &CandidateChain{
		ID:           candidateID,
		PeerSource:   peerAddress,
		ChainStore:   candidateStore,
		Headers:      headers,
		StartedAt:    time.Now(),
		ExpectedWork: chainHead.TotalWork,
	}
	candidate.expectedHeight.Store(chainHead.Height)
	candidate.currentHeight.Store(0)
	candidate.downloadStatus.Store(0) // downloading

	// Store in candidates map
	server.candidateChains.Store(candidateID, candidate)
	server.logf("Created candidate chain %s", candidateID)

	// Start download in background
	go DownloadCandidateChain(server, candidate)

	return nil
}

// DownloadCandidateChain downloads all blocks for a candidate chain
func DownloadCandidateChain(server *Server, candidate *CandidateChain) {
	defer func() {
		// Mark as complete or failed
		if candidate.downloadStatus.Load() == 0 { // still downloading
			candidate.downloadStatus.Store(1) // mark complete
		}
	}()

	server.logf("Starting block download for candidate %s", candidate.ID)

	// Download blocks in batches
	batchSize := uint64(100)
	startHeight := uint64(1)
	endHeight := candidate.expectedHeight.Load()

	for startHeight <= endHeight {
		count := batchSize
		if startHeight+count-1 > endHeight {
			count = endHeight - startHeight + 1
		}

		// Check if we should abort (better candidate appeared)
		if ShouldAbortDownload(server, candidate) {
			server.logf("Aborting download for candidate %s - better option found", candidate.ID)
			candidate.downloadStatus.Store(2) // mark failed
			return
		}

		server.logf("Candidate %s: downloading blocks %d-%d", candidate.ID, startHeight, startHeight+count-1)
		blocks, err := RequestBlocksFromPeer(server, candidate.PeerSource, startHeight, count)
		if err != nil {
			server.logf("Failed to download blocks for candidate %s: %v", candidate.ID, err)
			candidate.downloadStatus.Store(2) // mark failed
			return
		}

		// Add blocks to candidate's isolated chain store
		for _, block := range blocks {
			if err := candidate.ChainStore.AddBlock(block); err != nil {
				server.logf("Failed to add block to candidate %s: %v", candidate.ID, err)
				candidate.downloadStatus.Store(2) // mark failed
				return
			}
			candidate.currentHeight.Store(block.Header.Height)
		}

		server.logf("Candidate %s: added %d blocks, now at height %d",
			candidate.ID, len(blocks), candidate.currentHeight.Load())

		startHeight += count
	}

	server.logf("Candidate %s: download complete, evaluating for promotion", candidate.ID)

	// Download complete - evaluate for promotion
	EvaluateCandidateForPromotion(server, candidate)
}

// ShouldAbortDownload checks if we should abort this download for a better option
func ShouldAbortDownload(server *Server, candidate *CandidateChain) bool {
	bestOtherWork := "0"

	server.candidateChains.Range(func(key, value interface{}) bool {
		other := value.(*CandidateChain)
		if other.ID != candidate.ID && other.downloadStatus.Load() == 1 { // complete
			if blockchain.CompareWork(other.ExpectedWork, bestOtherWork) > 0 {
				bestOtherWork = other.ExpectedWork
			}
		}
		return true
	})

	// If there's a completed candidate with better work, abort this download
	return blockchain.CompareWork(bestOtherWork, candidate.ExpectedWork) > 0
}

// EvaluateCandidateForPromotion checks if candidate should become active chain
func EvaluateCandidateForPromotion(server *Server, candidate *CandidateChain) {
	// CRITICAL: Lock the chain for the entire validation and switch process
	// This prevents race conditions where the chain changes while we're validating
	server.config.Store.Lock()
	defer server.config.Store.Unlock()

	// Get current active chain work (now thread-safe under lock)
	currentChain, err := server.config.Store.GetChainUnsafe()
	if err != nil {
		server.logf("Failed to get current chain for comparison: %v", err)
		return
	}

	currentWork := "0"
	if len(currentChain.Blocks) > 0 {
		currentWork = currentChain.Blocks[len(currentChain.Blocks)-1].Header.TotalWork
	}

	// Compare work
	if blockchain.CompareWork(candidate.ExpectedWork, currentWork) > 0 {
		server.logf("Candidate %s has better work, validating before promotion", candidate.ID)

		// Get the candidate's complete chain
		candidateChain, err := candidate.ChainStore.GetChain()
		if err != nil {
			server.logf("Failed to get candidate chain for promotion: %v", err)
			return
		}

		// CRITICAL: Validate all blocks and transactions before switching
		server.logf("Validating candidate chain %s blocks and transactions", candidate.ID)

		// Create a fresh chain starting with genesis and rebuild state from scratch
		validationChain := &blockchain.Chain{
			Blocks:        []*blockchain.Block{candidateChain.Blocks[0]}, // Start with genesis
			AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
		}

		// Initialize with genesis account (FirstUser gets 10M coins)
		validationChain.AccountStates[blockchain.FirstUser] = &blockchain.AccountState{
			Address: blockchain.FirstUser,
			Balance: 10_000_000,
			Nonce:   0,
		}

		// Validate each block sequentially
		validationFailed := false
		for i := 1; i < len(candidateChain.Blocks); i++ {
			block := candidateChain.Blocks[i]
			if err := blockchain.ValidateAndApplyBlock(block, validationChain); err != nil {
				server.logf("Candidate chain %s validation failed at block %d: %v",
					candidate.ID, block.Header.Height, err)
				validationFailed = true
				break
			}
		}

		if validationFailed {
			server.logf("Candidate %s failed validation, discarding despite better work", candidate.ID)
			server.candidateChains.Delete(candidate.ID)
			return
		}

		server.logf("Candidate %s passed full validation, promoting to active chain", candidate.ID)

		// Atomic switch to new chain (fully validated, no race condition possible under lock)
		if err := server.config.Store.ReplaceChainUnsafe(candidateChain); err != nil {
			server.logf("Failed to replace active chain: %v", err)
			return
		}

		server.logf("Successfully promoted candidate %s to active chain (height %d)",
			candidate.ID, len(candidateChain.Blocks)-1)

		// Clean up this candidate
		server.candidateChains.Delete(candidate.ID)
	} else {
		server.logf("Candidate %s not better than current chain, discarding", candidate.ID)
	}
}

// ProcessBlock attempts to add a block to the main chain, handling orphans
// Returns a completion channel that will be closed when processing completes
// excludePeerAddr: if provided, this peer will be excluded from relay (used when block came from a peer)
func ProcessBlock(server *Server, block *blockchain.Block, excludePeerAddr ...string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		if block == nil {
			return
		}

		blockHash := blockchain.HashBlockHeader(&block.Header)

		// Determine exclude address for relay
		var excludeAddr string
		if len(excludePeerAddr) > 0 {
			excludeAddr = excludePeerAddr[0]
		}

		// Get current chain and make a deep copy for validation
		originalChain, err := server.config.Store.GetChain()
		if err != nil {
			server.logf("Failed to get chain for block %x: %v", blockHash[:8], err)
			return
		}

		// Make a deep copy to validate on without affecting the original
		chainCopy := originalChain.DeepCopy()
		if chainCopy == nil {
			server.logf("Failed to create chain copy for block %x", blockHash[:8])
			return
		}

		// Try to validate and apply block to the copy (this also adds the block to the chain)
		if err := blockchain.ValidateAndApplyBlock(block, chainCopy); err != nil {
			// Check if this is a missing parent error (orphan block)
			if missingParentErr, ok := err.(blockchain.ErrMissingParent); ok {
				server.logf("Block %x is orphan, missing parent %x. Adding to candidate blocks.",
					blockHash[:8], missingParentErr.Hash[:8])

				// Store in candidate blocks
				candidateBlock := &CandidateBlock{
					Block:        block,
					Source:       excludeAddr,
					ReceivedAt:   time.Now(),
					ParentNeeded: missingParentErr.Hash,
				}
				server.candidateBlocks.Store(blockHash, candidateBlock)

				// Request missing parent block from peers
				server.logf("Need to request parent block %x from peers", missingParentErr.Hash[:8])

				connectedPeers := server.peerManager.GetConnectedPeers()
				hashString := base64.StdEncoding.EncodeToString(missingParentErr.Hash[:])
				for _, peer := range connectedPeers {
					go func(peerAddr string) {
						_, _ = RequestBlockFromPeer(server, peerAddr, hashString)
					}(peer.Address)
				}
				return

				// Check if this is a chain switch request (fork detected)
			} else if details, ok := err.(blockchain.ErrSwitchChain); ok {
				server.logf("Block %x detected fork, need to check for chain reorganization", blockHash[:8])

				// Check if current chain has more work
				if blockchain.CompareWork(details.Block.Header.TotalWork, chainCopy.Blocks[len(chainCopy.Blocks)-1].Header.TotalWork) <= 0 {
					server.logf("Current chain has more total work, ignoring block %x", blockHash[:8])
					return
				}

				// Perform Chain Switch - fork has more work
				// Build new chain up to the fork point
				newBlocks := chainCopy.Blocks[:details.Block.Header.Height]
				newBlocks = append(newBlocks, details.Block)

				// Create new chain and rebuild account states from genesis
				newChain := &blockchain.Chain{
					Blocks:        newBlocks,
					AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
				}

				// Rebuild account states by processing all transactions
				for _, block := range newBlocks {
					for _, tx := range block.Transactions {
						blockchain.ValidateAndApplyTransaction(&tx, newChain.AccountStates)
					}
				}

				chainCopy = newChain
				server.logf("Reorganized Chain")

			} else {
				// Other validation errors
				server.logf("Block %x validation failed: %v", blockHash[:8], err)
				return
			}
		}

		// Block validation succeeded and was added to the copy, now atomically replace the chain
		if err := server.config.Store.ReplaceChain(chainCopy); err != nil {
			server.logf("Failed to replace chain with validated block %x: %v", blockHash[:8], err)
			return
		}

		// Block successfully added to main chain
		log.Printf("Block %x added to main chain", blockHash[:8])

		// Relay the block header to connected peers (headers-first approach)
		go func() { <-RelayBlockHeader(server, &block.Header, excludeAddr) }()

		// Try to connect any candidate blocks that might now be connectible
		tryConnectCandidateBlocks(server)
	}()

	return complete
}

// tryConnectCandidateBlocks attempts to connect candidate blocks to the main chain
func tryConnectCandidateBlocks(server *Server) {
	connected := true

	// Keep trying until no more candidate blocks can be connected
	for connected {
		connected = false

		// Get current chain state for each attempt
		originalChain, err := server.config.Store.GetChain()
		if err != nil {
			server.logf("Failed to get chain for candidate block connection: %v", err)
			return
		}

		var toProcess []blockchain.Hash32
		var blocksToProcess []*CandidateBlock

		// Collect candidate blocks to process
		server.candidateBlocks.Range(func(key, value interface{}) bool {
			blockHash := key.(blockchain.Hash32)
			candidateBlock := value.(*CandidateBlock)
			toProcess = append(toProcess, blockHash)
			blocksToProcess = append(blocksToProcess, candidateBlock)
			return true
		})

		// Try each candidate block
		for i, candidateBlock := range blocksToProcess {
			blockHash := toProcess[i]

			// Make a deep copy to validate on without affecting the original
			chainCopy := originalChain.DeepCopy()
			if chainCopy == nil {
				server.logf("Failed to create chain copy for candidate block %x", blockHash[:8])
				continue
			}

			// Try to validate and add this candidate block to the copy
			if err := blockchain.ValidateAndApplyBlock(candidateBlock.Block, chainCopy); err == nil {
				// Block validation succeeded, atomically replace the chain
				if err := server.config.Store.ReplaceChain(chainCopy); err != nil {
					server.logf("Failed to replace chain with candidate block %x: %v", blockHash[:8], err)
					continue
				}

				// Successfully connected!
				server.logf("Connected candidate block %x to main chain", blockHash[:8])

				// Remove from candidate blocks
				server.candidateBlocks.Delete(blockHash)
				connected = true

				// Update originalChain for next iteration
				originalChain = chainCopy

				// Don't continue iterating as we modified the state
				break
			}
		}
	}

	// Count remaining candidate blocks
	candidateCount := 0
	server.candidateBlocks.Range(func(key, value interface{}) bool {
		candidateCount++
		return true
	})

	if candidateCount > 0 {
		server.logf("Still have %d candidate blocks waiting for parents", candidateCount)
	}
}

// GetCandidateBlockCount returns the number of candidate blocks waiting for parents (for testing)
func GetCandidateBlockCount(server *Server) int {
	count := 0
	server.candidateBlocks.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
