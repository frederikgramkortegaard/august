package networking

import (
	"encoding/base64"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"

	"august/blockchain"
	store "august/storage"
)

// PeerRequestConfig defines parameters for Bitcoin-style peer request strategies
type PeerRequestConfig struct {
	// Maximum number of outstanding requests per peer (like Bitcoin Core's 16-32)
	MaxRequestsPerPeer int

	// Maximum number of peers to use for any single request operation
	MaxPeersForRequest int

	// Minimum number of peers to try before giving up
	MinPeersForRequest int

	// Request timeout for individual peer requests
	RequestTimeoutSec int

	// Whether to randomize peer selection order
	RandomizePeerOrder bool

	// Prefer peers that have been historically responsive
	PreferResponsivePeers bool
}

// DefaultPeerRequestConfig returns Bitcoin-inspired default configuration
func DefaultPeerRequestConfig() PeerRequestConfig {
	return PeerRequestConfig{
		MaxRequestsPerPeer:    16, // Conservative like Bitcoin Core
		MaxPeersForRequest:    5,  // Try up to 5 peers in parallel/sequence
		MinPeersForRequest:    2,  // Need at least 2 peers
		RequestTimeoutSec:     30, // 30 second timeout per peer
		RandomizePeerOrder:    true,
		PreferResponsivePeers: true,
	}
}

// selectPeersForRequest implements Bitcoin-style peer selection with randomization and limits
func selectPeersForRequest(server *Server) []*Peer {
	// Get all connected peers
	availablePeers := server.peerManager.GetConnectedPeers()
	if len(availablePeers) == 0 {
		return nil
	}

	// Get config from server
	config := server.config.PeerRequestConfig

	// Start with a copy to avoid modifying the original slice
	candidates := make([]*Peer, len(availablePeers))
	copy(candidates, availablePeers)

	// Randomize peer order if configured
	if config.RandomizePeerOrder {
		rand.Shuffle(len(candidates), func(i, j int) {
			candidates[i], candidates[j] = candidates[j], candidates[i]
		})
	}

	// TODO: Add peer performance/responsiveness tracking here
	// For now, just respect the configured limits

	maxPeers := config.MaxPeersForRequest
	if len(candidates) < maxPeers {
		maxPeers = len(candidates)
	}

	// Ensure we have at least the minimum required peers
	if maxPeers < config.MinPeersForRequest && len(candidates) >= config.MinPeersForRequest {
		maxPeers = config.MinPeersForRequest
	}

	server.logf("Selected %d peers for request from %d available (randomized: %v)",
		maxPeers, len(availablePeers), config.RandomizePeerOrder)

	return candidates[:maxPeers]
}

// RelayBlock broadcasts a block to all connected peers, optionally excluding specific peers
// Returns a completion channel that will be closed when the relay operation completes
func RelayBlock(server *Server, block *blockchain.Block, excludePeerAddrs ...string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		blockHash := blockchain.HashBlockHeader(&block.Header)

		// Create map for quick exclusion lookup
		excludeMap := make(map[string]bool)
		for _, addr := range excludePeerAddrs {
			excludeMap[addr] = true
		}

		// Add to recent blocks to prevent duplication when we receive it back from peers
		// (only when not excluding anyone, meaning this is a local block)
		if len(excludePeerAddrs) == 0 {
			server.recentBlocksMu.Lock()
			server.recentBlocks[blockHash] = time.Now()
			server.recentBlocksMu.Unlock()
		}

		if len(excludePeerAddrs) == 0 {
			server.logf("Relaying block %x to all peers", blockHash[:8])
		} else {
			server.logf("Relaying block %x to other peers (excluding %d peers)", blockHash[:8], len(excludePeerAddrs))
		}

		// Create the block payload
		blockPayload := NewBlockPayload{Block: block}
		msg, err := NewMessage(MessageTypeNewBlock, blockPayload)
		if err != nil {
			server.logf("Failed to create relay message for block %x: %v", blockHash[:8], err)
			return
		}

		// Send to all connected peers except the excluded ones
		relayCount := 0
		for _, peer := range server.peerManager.GetConnectedPeers() {
			if !excludeMap[peer.Address] && peer.Status == PeerConnected {
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

// RelayBlockHeader broadcasts a block header to all connected peers (headers-first), optionally excluding specific peers
// Returns a completion channel that will be closed when the relay operation completes
func RelayBlockHeader(server *Server, header *blockchain.BlockHeader, excludePeerAddrs ...string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		blockHash := blockchain.HashBlockHeader(header)

		// Create map for quick exclusion lookup
		excludeMap := make(map[string]bool)
		for _, addr := range excludePeerAddrs {
			excludeMap[addr] = true
		}

		// Add to recent blocks to prevent duplication when we receive it back from peers
		// (only when not excluding anyone, meaning this is a local header)
		if len(excludePeerAddrs) == 0 {
			server.recentBlocksMu.Lock()
			server.recentBlocks[blockHash] = time.Now()
			server.recentBlocksMu.Unlock()
		}

		if len(excludePeerAddrs) == 0 {
			server.logf("Relaying block header %x (height %d) to all peers", blockHash[:8], header.Height)
		} else {
			server.logf("Relaying block header %x (height %d) to other peers (excluding %d peers)",
				blockHash[:8], header.Height, len(excludePeerAddrs))
		}

		// Create the header payload
		headerPayload := NewBlockHeaderPayload{Header: *header}
		msg, err := NewMessage(MessageTypeNewBlockHeader, headerPayload)
		if err != nil {
			server.logf("Failed to create relay message for header %x: %v", blockHash[:8], err)
			return
		}

		// Send to all connected peers except the excluded ones
		relayCount := 0
		for _, peer := range server.peerManager.GetConnectedPeers() {
			if !excludeMap[peer.Address] && peer.Status == PeerConnected {
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

// RelayTransaction relays a transaction to all connected peers, optionally excluding specific peers
// Returns a completion channel that will be closed when the relay operation completes
func RelayTransaction(server *Server, tx *blockchain.Transaction, excludePeerAddrs ...string) <-chan struct{} {
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

		// Create map for quick exclusion lookup
		excludeMap := make(map[string]bool)
		for _, addr := range excludePeerAddrs {
			excludeMap[addr] = true
		}

		// Log what we're doing
		if len(excludePeerAddrs) == 0 {
			server.logf("Relaying transaction to all peers")
		} else {
			server.logf("Relaying transaction to other peers (excluding %d peers)", len(excludePeerAddrs))
		}

		// Send to all connected peers
		sentCount := 0
		for _, peer := range server.peerManager.GetConnectedPeers() {
			if !excludeMap[peer.Address] && peer.Status == PeerConnected {
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

// RequestPeers requests peers from multiple connected peers using smart selection
func RequestPeers(server *Server, maxPeers int) ([]string, error) {
	// Use smart peer selection
	selectedPeers := selectPeersForRequest(server)
	if len(selectedPeers) == 0 {
		return nil, fmt.Errorf("no suitable peers available")
	}

	// Request from selected peers in parallel
	type peerDiscoveryResponse struct {
		peers []string
		err   error
		peer  string
	}

	responses := make(chan peerDiscoveryResponse, len(selectedPeers))

	for _, peer := range selectedPeers {
		go func(p *Peer) {
			requestPayload := RequestPeersPayload{MaxPeers: maxPeers}
			msg, err := NewMessage(MessageTypeRequestPeers, requestPayload)
			if err != nil {
				responses <- peerDiscoveryResponse{nil, fmt.Errorf("failed to create request message: %w", err), p.Address}
				return
			}

			response, err := server.reqRespClient.SendRequest(p.Address, msg)
			if err != nil {
				responses <- peerDiscoveryResponse{nil, err, p.Address}
				return
			}

			responseMsg := response.(*Message)
			if responseMsg.Type != MessageTypeSharePeers {
				responses <- peerDiscoveryResponse{nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type), p.Address}
				return
			}

			var sharePayload SharePeersPayload
			if err := responseMsg.ParsePayload(&sharePayload); err != nil {
				responses <- peerDiscoveryResponse{nil, fmt.Errorf("failed to parse peers response: %w", err), p.Address}
				return
			}

			server.logf("Received %d peers from %s", len(sharePayload.Peers), p.Address)
			responses <- peerDiscoveryResponse{sharePayload.Peers, nil, p.Address}
		}(peer)
	}

	// Collect and aggregate all peer discoveries
	var allDiscoveredPeers []string
	peerSet := make(map[string]bool) // Deduplicate peers

	for i := 0; i < len(selectedPeers); i++ {
		resp := <-responses
		if resp.err == nil {
			for _, peerAddr := range resp.peers {
				if !peerSet[peerAddr] {
					peerSet[peerAddr] = true
					allDiscoveredPeers = append(allDiscoveredPeers, peerAddr)
				}
			}
		} else {
			server.logf("Peer discovery failed from %s: %v", resp.peer, resp.err)
		}
	}

	server.logf("Discovered %d unique peers from %d sources", len(allDiscoveredPeers), len(selectedPeers))
	return allDiscoveredPeers, nil
}

// RequestChainHead requests the current chain head from a peer
func RequestChainHead(server *Server, peer *Peer) (*ChainHeadPayload, error) {
	requestPayload := RequestChainHeadPayload{}

	msg, err := NewMessage(MessageTypeRequestChainHead, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain head request: %w", err)
	}

	response, err := server.reqRespClient.SendRequest(peer.Address, msg)
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

	server.logf("Received chain head from %s: height=%d, work=%s", peer.Address, headPayload.Height, headPayload.TotalWork)
	return &headPayload, nil
}

// RequestHeadersByHeight requests block headers in a range using smart peer selection
func RequestHeadersByHeight(server *Server, startHeight uint64, count uint64) ([]blockchain.BlockHeader, error) {
	// Use smart peer selection (handles getting connected peers internally)
	selectedPeers := selectPeersForRequest(server)
	if len(selectedPeers) == 0 {
		return nil, fmt.Errorf("no suitable peers available")
	}

	// Request from selected peers in parallel
	type headerResponse struct {
		headers []blockchain.BlockHeader
		err     error
		peer    string
	}

	responses := make(chan headerResponse, len(selectedPeers))

	for _, peer := range selectedPeers {
		go func(p *Peer) {
			requestPayload := RequestHeadersPayload{
				StartHeight: startHeight,
				Count:       count,
			}
			msg, err := NewMessage(MessageTypeRequestHeaders, requestPayload)
			if err != nil {
				responses <- headerResponse{nil, fmt.Errorf("failed to create headers request: %w", err), p.Address}
				return
			}

			response, err := server.reqRespClient.SendRequest(p.Address, msg)
			if err != nil {
				responses <- headerResponse{nil, err, p.Address}
				return
			}

			responseMsg := response.(*Message)
			if responseMsg.Type != MessageTypeHeaders {
				responses <- headerResponse{nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type), p.Address}
				return
			}

			var headersPayload HeadersPayload
			if err := responseMsg.ParsePayload(&headersPayload); err != nil {
				responses <- headerResponse{nil, fmt.Errorf("failed to parse headers response: %w", err), p.Address}
				return
			}

			server.logf("Received %d headers from %s starting at height %d", len(headersPayload.Headers), p.Address, startHeight)
			responses <- headerResponse{headersPayload.Headers, nil, p.Address}
		}(peer)
	}

	// Collect responses
	var successfulResponses []headerResponse
	for i := 0; i < len(selectedPeers); i++ {
		resp := <-responses
		if resp.err == nil {
			successfulResponses = append(successfulResponses, resp)
		} else {
			server.logf("Header request failed from %s: %v", resp.peer, resp.err)
		}
	}

	if len(successfulResponses) == 0 {
		return nil, fmt.Errorf("all peers failed to provide headers starting at height %d", startHeight)
	}

	// For now, just return the first successful response
	// TODO: Add consensus verification later if needed
	server.logf("Got headers from %d/%d peers, using first successful", len(successfulResponses), len(selectedPeers))
	return successfulResponses[0].headers, nil
}

// RequestHeadersByHash requests specific headers by their hashes using smart peer selection
func RequestHeadersByHash(server *Server, blockHashes []string) ([]blockchain.BlockHeader, error) {
	// Use smart peer selection (handles getting connected peers internally)
	selectedPeers := selectPeersForRequest(server)
	if len(selectedPeers) == 0 {
		return nil, fmt.Errorf("no suitable peers available")
	}

	// Request from selected peers in parallel
	type headerResponse struct {
		headers []blockchain.BlockHeader
		err     error
		peer    string
	}

	responses := make(chan headerResponse, len(selectedPeers))

	for _, peer := range selectedPeers {
		go func(p *Peer) {
			requestPayload := RequestHeadersPayload{
				Hashes: blockHashes,
			}
			msg, err := NewMessage(MessageTypeRequestHeaders, requestPayload)
			if err != nil {
				responses <- headerResponse{nil, fmt.Errorf("failed to create headers request: %w", err), p.Address}
				return
			}

			response, err := server.reqRespClient.SendRequest(p.Address, msg)
			if err != nil {
				responses <- headerResponse{nil, err, p.Address}
				return
			}

			responseMsg := response.(*Message)
			if responseMsg.Type != MessageTypeHeaders {
				responses <- headerResponse{nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type), p.Address}
				return
			}

			var headersPayload HeadersPayload
			if err := responseMsg.ParsePayload(&headersPayload); err != nil {
				responses <- headerResponse{nil, fmt.Errorf("failed to parse headers response: %w", err), p.Address}
				return
			}

			server.logf("Received %d headers from %s by hash", len(headersPayload.Headers), p.Address)
			responses <- headerResponse{headersPayload.Headers, nil, p.Address}
		}(peer)
	}

	// Collect responses
	var successfulResponses []headerResponse
	for i := 0; i < len(selectedPeers); i++ {
		resp := <-responses
		if resp.err == nil {
			successfulResponses = append(successfulResponses, resp)
		} else {
			server.logf("Header request failed from %s: %v", resp.peer, resp.err)
		}
	}

	if len(successfulResponses) == 0 {
		return nil, fmt.Errorf("all peers failed to provide headers for requested hashes")
	}

	// For now, just return the first successful response
	// TODO : Add consensus verification later if needed
	// TODO : Handle dropout etc.
	server.logf("Got headers from %d/%d peers, using first successful", len(successfulResponses), len(selectedPeers))
	return successfulResponses[0].headers, nil
}

// RequestBlocksByHash requests specific blocks by their hashes using Bitcoin-style peer selection
func RequestBlocksByHash(server *Server, blockHashes []string) ([]*blockchain.Block, error) {
	// Use smart peer selection (handles getting connected peers internally)
	selectedPeers := selectPeersForRequest(server)
	if len(selectedPeers) == 0 {
		return nil, fmt.Errorf("no suitable peers available")
	}

	// Request from selected peers in parallel
	type peerResponse struct {
		blocks []*blockchain.Block
		err    error
		peer   string
	}

	responses := make(chan peerResponse, len(selectedPeers))

	for _, peer := range selectedPeers {
		go func(p *Peer) {
			requestPayload := RequestBlocksPayload{Hashes: blockHashes}
			msg, err := NewMessage(MessageTypeRequestBlocks, requestPayload)
			if err != nil {
				responses <- peerResponse{nil, fmt.Errorf("failed to create blocks request: %w", err), p.Address}
				return
			}

			response, err := server.reqRespClient.SendRequest(p.Address, msg)
			if err != nil {
				responses <- peerResponse{nil, err, p.Address}
				return
			}

			responseMsg := response.(*Message)
			if responseMsg.Type != MessageTypeBlocks {
				responses <- peerResponse{nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type), p.Address}
				return
			}

			var blocksPayload BlocksPayload
			if err := responseMsg.ParsePayload(&blocksPayload); err != nil {
				responses <- peerResponse{nil, fmt.Errorf("failed to parse blocks response: %w", err), p.Address}
				return
			}

			server.logf("Received %d requested blocks from %s", len(blocksPayload.Blocks), p.Address)
			responses <- peerResponse{blocksPayload.Blocks, nil, p.Address}
		}(peer)
	}

	// Collect responses
	var successfulResponses []peerResponse
	for i := 0; i < len(selectedPeers); i++ {
		resp := <-responses
		if resp.err == nil {
			successfulResponses = append(successfulResponses, resp)
		} else {
			server.logf("Failed to get blocks from %s: %v", resp.peer, resp.err)
		}
	}

	if len(successfulResponses) == 0 {
		return nil, fmt.Errorf("all peers failed to provide blocks")
	}

	// For now, just return the first successful response
	// TODO: Add consensus verification later if needed
	server.logf("Got blocks from %d/%d peers, using first successful", len(successfulResponses), len(selectedPeers))
	return successfulResponses[0].blocks, nil
}

// EvaluateChainHead performs headers-first evaluation of a peer's chain
func EvaluateChainHead(server *Server, peer *Peer, peerChainHead *ChainHeadPayload) error {
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
		peer.Address, ourHeight, ourWork, peerChainHead.Height, peerChainHead.TotalWork)

	// Quick work comparison - if peer is not better, skip everything
	if blockchain.CompareWork(peerChainHead.TotalWork, ourWork) <= 0 {
		server.logf("Peer %s chain not better than ours, skipping", peer.Address)
		return nil
	}

	// Phase 1: Download headers first from multiple peers (lightweight evaluation)
	server.logf("Peer chain looks better, downloading headers from multiple peers (triggered by %s)", peer.Address)

	// Request headers using smart peer selection
	headers, err := RequestHeadersByHeight(server, 1, peerChainHead.Height)
	if err != nil {
		return fmt.Errorf("failed to download headers from %s: %w", peer.Address, err)
	}

	// Validate header chain before proceeding
	if !blockchain.ValidateHeaderChain(headers) {
		server.logf("Header chain from %s invalid, rejecting", peer.Address)
		return fmt.Errorf("invalid header chain from %s", peer.Address)
	}

	server.logf("Downloaded and validated %d headers from %s", len(headers), peer.Address)

	// Double-check work calculation from headers
	finalHeaderWork := headers[len(headers)-1].TotalWork
	if blockchain.CompareWork(finalHeaderWork, ourWork) <= 0 {
		server.logf("Header chain work not better after validation, skipping")
		return nil
	}

	// Phase 2: Headers look good, start candidate chain download
	server.logf("Headers validated and better, starting candidate chain download")
	return StartCandidateChainDownload(server, peer, headers, peerChainHead)
}

// StartCandidateChainDownload creates a candidate chain and downloads blocks
func StartCandidateChainDownload(server *Server, peer *Peer, headers []blockchain.BlockHeader, chainHead *ChainHeadPayload) error {
	// Generate candidate ID
	candidateID := fmt.Sprintf("%s-%d-%s", peer.Address, chainHead.Height, chainHead.HeadHash[:16])

	// Create candidate chain with isolated storage
	candidateStore := store.NewMemoryChainStore()

	candidate := &CandidateChain{
		ID:           candidateID,
		PeerSource:   peer.Address,
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

		// Convert headers to hashes for this batch
		var blockHashes []string
		for _, header := range candidate.Headers {
			if header.Height >= startHeight && header.Height < startHeight+count {
				hash := blockchain.HashBlockHeader(&header)
				hashStr := base64.StdEncoding.EncodeToString(hash[:])
				blockHashes = append(blockHashes, hashStr)
			}
		}

		server.logf("Candidate %s: downloading %d blocks %d-%d from multiple peers", candidate.ID, len(blockHashes), startHeight, startHeight+count-1)

		blocks, err := RequestBlocksByHash(server, blockHashes)
		if err != nil {
			server.logf("Failed to download blocks for candidate %s: %v", candidate.ID, err)
			candidate.downloadStatus.Store(2) // mark failed
			return
		}

		server.logf("Candidate %s: downloaded %d blocks from peers", candidate.ID, len(blocks))

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

		if err := blockchain.ValidateCompleteChain(candidateChain); err != nil {
			server.logf("Candidate %s failed validation, discarding despite better work: %v", candidate.ID, err)
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

				// Request missing parent block from multiple peers
				server.logf("Need to request parent block %x from peers", missingParentErr.Hash[:8])

				connectedPeers := server.peerManager.GetConnectedPeers()
				if len(connectedPeers) > 0 {
					hashString := base64.StdEncoding.EncodeToString(missingParentErr.Hash[:])
					go func() {
						blocks, err := RequestBlocksByHash(server, []string{hashString})
						if err != nil {
							server.logf("Failed to request parent block %x from peers: %v", missingParentErr.Hash[:8], err)
							return
						}

						if len(blocks) == 0 {
							server.logf("No parent block returned for %x from peers", missingParentErr.Hash[:8])
							return
						}

						// Process the parent block - this should connect orphan blocks
						<-ProcessBlock(server, blocks[0])
					}()
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

// SendPongResponse sends a pong response to a ping
func (s *Server) SendPongResponse(conn net.Conn) {
	pongMsg, err := NewMessage(MessageTypePong, nil)
	if err == nil {
		s.sendMessage(conn, pongMsg)
	}
}

// SendPeerList sends a list of peers in response to a request
func (s *Server) SendPeerList(conn net.Conn, msg *Message, requesterAddr string) {
	s.logf("Peer %s requested peer list", requesterAddr)

	// Get list of connected peers (excluding the requester)
	peers := s.peerManager.GetConnectedPeers()
	var peerAddresses []string
	for _, p := range peers {
		if p.Address != requesterAddr && p.Status == PeerConnected {
			peerAddresses = append(peerAddresses, p.Address)
		}
	}

	s.logf("Sharing %d peers with %s", len(peerAddresses), requesterAddr)

	sharePayload := SharePeersPayload{
		Peers: peerAddresses,
	}

	shareMsg, err := NewMessage(MessageTypeSharePeers, sharePayload)
	if err != nil {
		s.logf("Failed to create share peers message: %v", err)
		return
	}

	// Set reply info if this is a response
	if msg.RequestID != "" {
		shareMsg.ReplyTo = msg.RequestID
	}

	s.sendMessage(conn, shareMsg)
}

// SendBlockResponse sends a block in response to a request
func (s *Server) SendBlockResponse(conn net.Conn, msg *Message, peer *Peer) {
	var payload RequestBlockPayload
	if err := msg.ParsePayload(&payload); err != nil {
		s.logf("Failed to decode request block payload: %v", err)
		return
	}

	s.logf("Peer %s requested block %s", peer.Address, payload.BlockHash)

	// Get the block from our chain store
	chain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get chain: %v", err)
		return
	}

	// Find the requested block
	var foundBlock *blockchain.Block
	for _, block := range chain.Blocks {
		blockHash := blockchain.HashBlockHeader(&block.Header)
		blockHashStr := base64.StdEncoding.EncodeToString(blockHash[:])
		if blockHashStr == payload.BlockHash {
			foundBlock = block
			break
		}
	}

	if foundBlock == nil {
		s.logf("Block %s not found for peer %s", payload.BlockHash, peer.Address)
		return
	}

	// Create response message with the block
	response, err := NewMessage(MessageTypeNewBlock, NewBlockPayload{Block: foundBlock})
	if err != nil {
		s.logf("Failed to create block response: %v", err)
		return
	}

	// Set reply info to correlate with request
	response.ReplyTo = msg.RequestID

	if err := s.sendMessage(conn, response); err != nil {
		s.logf("Failed to send block to %s: %v", peer.Address, err)
	} else {
		s.logf("Sent block %s to peer %s", payload.BlockHash, peer.Address)
	}
}

// SendChainHeadResponse sends chain head information in response to a request
func (s *Server) SendChainHeadResponse(conn net.Conn, msg *Message, peer *Peer) {
	s.logf("Peer %s requested chain head", peer.Address)

	chain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get chain: %v", err)
		return
	}

	if len(chain.Blocks) == 0 {
		s.logf("No blocks in chain to share with %s", peer.Address)
		return
	}

	// Get our chain head
	headBlock := chain.Blocks[len(chain.Blocks)-1]
	headHash := blockchain.HashBlockHeader(&headBlock.Header)

	headPayload := ChainHeadPayload{
		HeadHash:  base64.StdEncoding.EncodeToString(headHash[:]),
		Height:    headBlock.Header.Height,
		TotalWork: headBlock.Header.TotalWork,
		Header:    headBlock.Header,
	}

	response, err := NewMessage(MessageTypeChainHead, headPayload)
	if err != nil {
		s.logf("Failed to create chain head response: %v", err)
		return
	}

	// Set reply info to correlate with request
	if msg.RequestID != "" {
		response.ReplyTo = msg.RequestID
	}

	if err := s.sendMessage(conn, response); err != nil {
		s.logf("Failed to send chain head to %s: %v", peer.Address, err)
	} else {
		s.logf("Sent chain head (height %d) to peer %s", headBlock.Header.Height, peer.Address)
	}
}

// ProcessHandshake handles the business logic for processing a handshake
func (s *Server) ProcessHandshake(msg *Message, peer *Peer, conn net.Conn) {
	var handshake HandshakePayload
	if err := msg.ParsePayload(&handshake); err != nil {
		s.logf("Failed to decode handshake payload from %s: %v", peer.Address, err)
		return
	}

	// Construct the proper peer address using IP from connection + ListenPort from handshake
	remoteAddr := conn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		s.logf("Failed to parse remote address %s: %v", remoteAddr, err)
		return
	}

	properPeerAddr := net.JoinHostPort(host, handshake.ListenPort)
	s.logf("Received handshake from %s (node %s), actual peer address: %s",
		peer.Address, handshake.NodeID, properPeerAddr)

	// Check for duplicate connections (bidirectional connection detection)
	s.peerManager.mu.Lock()
	existingPeer, exists := s.peerManager.peers[properPeerAddr]
	if exists && existingPeer != peer {
		// We have a duplicate connection!
		// Determine which connection to keep based on address comparison
		localListenAddr := net.JoinHostPort(host, s.config.Port)

		s.logf("Duplicate connection detected: local=%s, remote=%s", localListenAddr, properPeerAddr)

		if strings.Compare(localListenAddr, properPeerAddr) < 0 {
			// Our address is "smaller", keep incoming connection (this one)
			s.logf("Keeping incoming connection from %s (lower address)", properPeerAddr)

			// Close the existing outgoing connection
			existingPeer.Status = PeerDisconnected
			// Remove old peer and replace with this one
			delete(s.peerManager.peers, properPeerAddr)
		} else {
			// Remote address is "smaller", keep existing outgoing connection
			s.logf("Rejecting incoming connection from %s (keeping outgoing)", properPeerAddr)
			s.peerManager.mu.Unlock()

			// Close this connection and return
			conn.Close()
			peer.Status = PeerDisconnected
			return
		}
	}

	// Update peer information
	peer.ID = handshake.NodeID
	peer.Status = PeerConnected

	// If the address changed (e.g., from connection address to proper listen address)
	if peer.Address != properPeerAddr {
		// Update the peer connection mapping
		s.peerConnectionsMu.Lock()
		delete(s.peerConnections, peer.Address)
		s.peerConnections[properPeerAddr] = conn
		s.peerConnectionsMu.Unlock()

		// Update peer address
		peer.Address = properPeerAddr
	}

	// Update peer in manager with proper address
	s.peerManager.peers[properPeerAddr] = peer
	s.peerManager.mu.Unlock()

	s.logf("Handshake completed with %s (node: %s)",
		properPeerAddr, handshake.NodeID)
}

// ProcessChainHead handles chain head messages and initiates sync if needed
func (s *Server) ProcessChainHead(msg *Message, peer *Peer) {
	var headPayload ChainHeadPayload
	if err := msg.ParsePayload(&headPayload); err != nil {
		s.logf("Failed to decode chain head payload from %s: %v", peer.Address, err)
		return
	}

	s.logf("Received chain head from %s: height=%d, hash=%s",
		peer.Address, headPayload.Height, headPayload.HeadHash[:16])

	// Compare with our chain
	ourChain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get our chain for comparison: %v", err)
		return
	}

	ourHeight := uint64(0)
	if len(ourChain.Blocks) > 0 {
		ourHeight = ourChain.Blocks[len(ourChain.Blocks)-1].Header.Height
	}

	// If peer is significantly ahead (more than 1 block), initiate IBD
	if headPayload.Height > ourHeight+1 {
		s.logf("Peer %s is ahead by %d blocks, initiating IBD",
			peer.Address, headPayload.Height-ourHeight)

		// Start headers-first evaluation in background
		go func() {
			if err := EvaluateChainHead(s, peer, &headPayload); err != nil {
				s.logf("Chain evaluation failed from %s: %v", peer.Address, err)
			}
		}()
	} else if headPayload.Height == ourHeight+1 {
		s.logf("Peer %s has next block, requesting it", peer.Address)

		// Request just the next block (prefer the peer who advertised it)
		go func() {
			requestPeers := []*Peer{peer}
			// Add other peers as backup
			otherPeers := s.peerManager.GetConnectedPeers()
			for _, otherPeer := range otherPeers {
				if otherPeer.Address != peer.Address && len(requestPeers) < 3 {
					requestPeers = append(requestPeers, otherPeer)
				}
			}

			blocks, err := RequestBlocksByHash(s, []string{headPayload.HeadHash})
			if err != nil {
				s.logf("Failed to get next block from peers: %v", err)
				return
			}

			if len(blocks) == 0 {
				s.logf("No next block returned from peers")
				return
			}

			// Process the single block
			<-ProcessBlock(s, blocks[0], peer.Address)
		}()
	}
}

// ProcessNewBlockHeader handles the business logic for processing a new block header
func (s *Server) ProcessNewBlockHeader(msg *Message, peer *Peer) {
	var headerPayload NewBlockHeaderPayload
	if err := msg.ParsePayload(&headerPayload); err != nil {
		s.logf("Failed to parse block header payload: %v", err)
		return
	}

	header := &headerPayload.Header
	blockHash := blockchain.HashBlockHeader(header)

	// Check if we already have this block header in our recent blocks (deduplication)
	s.recentBlocksMu.Lock()
	if addedTime, seen := s.recentBlocks[blockHash]; seen {
		if time.Now().Sub(addedTime) <= s.recentBlocksTTL {
			s.recentBlocksMu.Unlock()
			s.logf("Ignoring duplicate block header: %x", blockHash[:8])
			return
		}
	}
	s.recentBlocks[blockHash] = time.Now()
	s.recentBlocksMu.Unlock()

	s.logf("Received new block header %x (height %d) from peer %s",
		blockHash[:8], header.Height, peer.Address)

	// Get current chain and let blockchain package decide if we want this block
	ourChain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get our chain: %v", err)
		return
	}

	// Use blockchain package to evaluate if we should request this block
	evaluation := blockchain.EvaluateBlockHeaderForSync(header, ourChain)

	if evaluation.ShouldRequest {
		s.logf("Requesting full block %x: %s", blockHash[:8], evaluation.Reason)

		// Request the full block
		hashString := base64.StdEncoding.EncodeToString(blockHash[:])
		go func() {
			// Use smart peer selection internally
			blocks, err := RequestBlocksByHash(s, []string{hashString})
			if err != nil {
				s.logf("Failed to request block %x from peers: %v", blockHash[:8], err)
				return
			}

			if len(blocks) == 0 {
				s.logf("No block returned for %x from %s", blockHash[:8], peer.Address)
				return
			}

			// Process the full block
			<-ProcessBlock(s, blocks[0], peer.Address)
		}()
	} else {
		s.logf("Ignoring block header %x (height %d, %s)", blockHash[:8], header.Height, evaluation.Reason)
	}

	// Always relay the header to other peers (except sender)
	go func() { <-RelayBlockHeader(s, header, peer.Address) }()
}

// ProcessHeaders handles received headers and integrates them with headers-first sync
func (s *Server) ProcessHeaders(msg *Message, peer *Peer) {
	var headersPayload HeadersPayload
	if err := msg.ParsePayload(&headersPayload); err != nil {
		s.logf("Failed to parse headers: %v", err)
		return
	}

	s.logf("Received %d headers from %s", len(headersPayload.Headers), peer.Address)

	if len(headersPayload.Headers) == 0 {
		return
	}

	// Validate the header chain
	if !blockchain.ValidateHeaderChain(headersPayload.Headers) {
		s.logf("Invalid header chain from %s, rejecting", peer.Address)
		return
	}

	// Check if these headers represent a better chain than ours
	ourChain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get our chain: %v", err)
		return
	}

	ourWork := "0"
	if len(ourChain.Blocks) > 0 {
		ourWork = ourChain.Blocks[len(ourChain.Blocks)-1].Header.TotalWork
	}

	finalHeaderWork := headersPayload.Headers[len(headersPayload.Headers)-1].TotalWork
	if blockchain.CompareWork(finalHeaderWork, ourWork) <= 0 {
		s.logf("Headers from %s not better than our chain, ignoring", peer.Address)
		return
	}

	s.logf("Headers from %s represent better chain, starting candidate download", peer.Address)

	// Create a mock chain head payload from the final header
	finalHeader := headersPayload.Headers[len(headersPayload.Headers)-1]
	finalHash := blockchain.HashBlockHeader(&finalHeader)
	chainHead := &ChainHeadPayload{
		HeadHash:  base64.StdEncoding.EncodeToString(finalHash[:]),
		Height:    finalHeader.Height,
		TotalWork: finalHeader.TotalWork,
		Header:    finalHeader,
	}

	// Start candidate chain download using existing headers-first system
	if err := StartCandidateChainDownload(s, peer, headersPayload.Headers, chainHead); err != nil {
		s.logf("Failed to start candidate chain download from %s: %v", peer.Address, err)
	}
}

// ProcessBlocks handles received blocks
func (s *Server) ProcessBlocks(msg *Message, peer *Peer) {
	var blocksPayload BlocksPayload
	if err := msg.ParsePayload(&blocksPayload); err != nil {
		s.logf("Failed to parse blocks: %v", err)
		return
	}

	s.logf("Received %d blocks from %s", len(blocksPayload.Blocks), peer.Address)

	// Process each block sequentially
	for _, block := range blocksPayload.Blocks {
		blockHash := blockchain.HashBlockHeader(&block.Header)
		s.logf("Processing batch block %x from %s", blockHash[:8], peer.Address)

		// Process block through the normal pipeline
		go func(b *blockchain.Block) {
			<-ProcessBlock(s, b, peer.Address)
		}(block)
	}
}

// ProcessNewBlock handles incoming block announcements
func (s *Server) ProcessNewBlock(msg *Message, peer *Peer) {
	var blockPayload NewBlockPayload
	if err := msg.ParsePayload(&blockPayload); err != nil {
		s.logf("Failed to parse block payload: %v", err)
		return
	}

	// Mitigate Broadcast Storm by keeping list of blocks to ignore
	blockHash := blockchain.HashBlockHeader(&blockPayload.Block.Header)
	s.recentBlocksMu.Lock()
	if addedTime, seen := s.recentBlocks[blockHash]; seen {
		if time.Now().Sub(addedTime) <= s.recentBlocksTTL {
			s.recentBlocksMu.Unlock()
			s.logf("Ignoring new block: %x because its in recentblocks", blockHash[:8])
			return
		}
	}
	s.recentBlocks[blockHash] = time.Now()
	s.recentBlocksMu.Unlock()

	s.logf("Received new block %x from peer %s", blockHash[:8], peer.Address)

	// Process the block using the service layer (exclude the sender from relay)
	go func() { <-ProcessBlock(s, blockPayload.Block, peer.Address) }()
}

// ProcessPong handles pong responses
func (s *Server) ProcessPong(peer *Peer) {
	s.logf("Received pong from peer %s", peer.Address)
}

// ProcessSharedPeers handles received peer lists
func (s *Server) ProcessSharedPeers(msg *Message, peer *Peer) {
	var sharePayload SharePeersPayload
	if err := msg.ParsePayload(&sharePayload); err != nil {
		s.logf("Failed to parse shared peers: %v", err)
		return
	}

	s.logf("Received %d peers from %s", len(sharePayload.Peers), peer.Address)
	// The discovery component handles adding these peers
}

// ProcessNewTransaction handles incoming transaction announcements
func (s *Server) ProcessNewTransaction(msg *Message, peer *Peer) {
	var txPayload NewTxPayload
	if err := msg.ParsePayload(&txPayload); err != nil {
		s.logf("Failed to parse transaction payload: %v", err)
		return
	}

	s.logf("Received new transaction from peer %s", peer.Address)

	// TODO: Add transaction to mempool and validate
	// For now, just relay to other peers
	go func() { <-RelayTransaction(s, txPayload.Transaction, peer.Address) }()
}

// SendHeadersResponse sends headers in response to a request
func (s *Server) SendHeadersResponse(conn net.Conn, msg *Message, peer *Peer) {
	var payload RequestHeadersPayload
	if err := msg.ParsePayload(&payload); err != nil {
		s.logf("Failed to decode request headers payload: %v", err)
		return
	}

	chain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get chain: %v", err)
		return
	}

	var headers []blockchain.BlockHeader

	if len(payload.Hashes) > 0 {
		// Hash-based request
		s.logf("Peer %s requested %d specific headers by hash", peer.Address, len(payload.Hashes))

		hashMap := make(map[string]bool)
		for _, hashStr := range payload.Hashes {
			hashMap[hashStr] = true
		}

		for _, block := range chain.Blocks {
			blockHash := blockchain.HashBlockHeader(&block.Header)
			blockHashStr := base64.StdEncoding.EncodeToString(blockHash[:])
			if hashMap[blockHashStr] {
				headers = append(headers, block.Header)
			}
		}
	} else {
		// Height-based request
		s.logf("Peer %s requested %d headers starting from height %d",
			peer.Address, payload.Count, payload.StartHeight)

		for _, block := range chain.Blocks {
			if block.Header.Height >= payload.StartHeight &&
				block.Header.Height < payload.StartHeight+payload.Count {
				headers = append(headers, block.Header)
			}
		}
	}

	s.logf("Sending %d headers to %s", len(headers), peer.Address)

	response, err := NewMessage(MessageTypeHeaders, HeadersPayload{Headers: headers})
	if err != nil {
		s.logf("Failed to create headers response: %v", err)
		return
	}

	// Set reply info to correlate with request
	response.ReplyTo = msg.RequestID

	if err := s.sendMessage(conn, response); err != nil {
		s.logf("Failed to send headers to %s: %v", peer.Address, err)
	}
}

// SendBlocksResponse sends blocks in response to a request
func (s *Server) SendBlocksResponse(conn net.Conn, msg *Message, peer *Peer) {
	var payload interface{}

	// Determine request type
	if msg.Type == MessageTypeRequestBlocks {
		var reqPayload RequestBlocksPayload
		if err := msg.ParsePayload(&reqPayload); err != nil {
			s.logf("Failed to decode request blocks payload: %v", err)
			return
		}
		payload = reqPayload
	}

	chain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get chain: %v", err)
		return
	}

	var blocks []*blockchain.Block

	switch p := payload.(type) {
	case RequestBlocksPayload:
		if len(p.Hashes) > 0 {
			// Hash-based request
			s.logf("Peer %s requested %d specific blocks by hash",
				peer.Address, len(p.Hashes))

			// Create a map for quick lookup
			hashMap := make(map[string]bool)
			for _, hashStr := range p.Hashes {
				hashMap[hashStr] = true
			}

			for _, block := range chain.Blocks {
				blockHash := blockchain.HashBlockHeader(&block.Header)
				blockHashStr := base64.StdEncoding.EncodeToString(blockHash[:])
				if hashMap[blockHashStr] {
					blocks = append(blocks, block)
				}
			}
		} else {
			// Height-based request
			s.logf("Peer %s requested %d blocks starting from height %d",
				peer.Address, p.Count, p.StartHeight)

			for _, block := range chain.Blocks {
				if block.Header.Height >= p.StartHeight &&
					block.Header.Height < p.StartHeight+p.Count {
					blocks = append(blocks, block)
				}
			}
		}
	}

	s.logf("Sending %d blocks to %s", len(blocks), peer.Address)

	response, err := NewMessage(MessageTypeBlocks, BlocksPayload{Blocks: blocks})
	if err != nil {
		s.logf("Failed to create blocks response: %v", err)
		return
	}

	// Set reply info to correlate with request
	response.ReplyTo = msg.RequestID

	if err := s.sendMessage(conn, response); err != nil {
		s.logf("Failed to send blocks to %s: %v", peer.Address, err)
	}
}
