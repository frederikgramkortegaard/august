package networking

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"time"

	"august/blockchain"
	"august/config"
	"august/consensus"
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
		MaxPeersForRequest:    20, // Try up to 20 peers in parallel/sequence
		MinPeersForRequest:    1,  // Can sync from 1 peer (like Bitcoin)
		RequestTimeoutSec:     30, // 30 second timeout per peer
		RandomizePeerOrder:    true,
		PreferResponsivePeers: true,
	}
}

// selectPeersForRequest implements Bitcoin-style peer selection with randomization and limits
func selectPeersForRequest(server *Server) []*Peer {
	// Get all connected peers
	availablePeers := server.GetConnectedPeers()
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

		blockHash := block.Header.GetHash()

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
		for _, peer := range server.GetConnectedPeers() {
			if !excludeMap[peer.Address] && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := server.SendNotification(peer, msg); err != nil {
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

// RelayTransaction broadcasts a transaction to all connected peers, optionally excluding specific peers
// Returns a completion channel that will be closed when the relay operation completes
func RelayTransaction(server *Server, tx *blockchain.Transaction, excludePeerAddrs ...string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		txHash := tx.GetHash()

		// Mark this transaction as seen (for deduplication)
		server.MarkTransactionSeen(txHash)

		// Create map for quick exclusion lookup
		excludeMap := make(map[string]bool)
		for _, addr := range excludePeerAddrs {
			excludeMap[addr] = true
		}

		// Get list of connected peers
		peers := server.GetConnectedPeers()

		// Track how many we actually relay to
		relayCount := 0

		for _, peer := range peers {
			// Skip excluded peers (typically the one who sent us the transaction)
			if excludeMap[peer.Address] {
				continue
			}

			// Send transaction to peer
			msg, err := NewMessage(MessageTypeNewTx, tx)
			if err != nil {
				server.logf("Failed to create transaction message: %v", err)
				continue
			}

			err = server.SendNotification(peer, msg)
			if err != nil {
				server.logf("Failed to relay transaction %x to peer %s: %v", txHash[:8], peer.Address, err)
			} else {
				relayCount++
			}
		}

		if relayCount > 0 {
			server.logf("Relayed transaction %x to %d peers", txHash[:8], relayCount)
		}
	}()

	return complete
}

// RelayBlockHeader broadcasts a block header to all connected peers (headers-first), optionally excluding specific peers
// Returns a completion channel that will be closed when the relay operation completes
func RelayBlockHeader(server *Server, header *blockchain.BlockHeader, excludePeerAddrs ...string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		blockHash := header.GetHash()

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
		for _, peer := range server.GetConnectedPeers() {
			if !excludeMap[peer.Address] && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := server.SendNotification(peer, msg); err != nil {
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

			response, err := server.SendRequest(p, msg)
			if err != nil {
				responses <- peerDiscoveryResponse{nil, err, p.Address}
				return
			}

			responseMsg := response
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
func RequestChainHead(server *Server, peer *Peer) (*consensus.ChainHeadPayload, error) {
	requestPayload := RequestChainHeadPayload{}

	msg, err := NewMessage(MessageTypeRequestChainHead, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain head request: %w", err)
	}

	response, err := server.SendRequest(peer, msg)
	if err != nil {
		return nil, err
	}

	responseMsg := response
	if responseMsg.Type != MessageTypeChainHead {
		return nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type)
	}

	var headPayload consensus.ChainHeadPayload
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

			response, err := server.SendRequest(p, msg)
			if err != nil {
				responses <- headerResponse{nil, err, p.Address}
				return
			}

			responseMsg := response
			if responseMsg.Type != MessageTypeHeaders {
				responses <- headerResponse{nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type), p.Address}
				return
			}

			var headersPayload HeadersPayload
			if err := responseMsg.ParsePayload(&headersPayload); err != nil {
				responses <- headerResponse{nil, fmt.Errorf("failed to parse headers response: %w", err), p.Address}
				return
			}

			// Validate each header structure
			for i, header := range headersPayload.Headers {
				if err := blockchain.ValidateHeaderStructure(&header); err != nil {
					responses <- headerResponse{nil, fmt.Errorf("invalid header %d from peer: %w", i, err), p.Address}
					return
				}
			}

			server.logf("Received %d valid headers from %s starting at height %d", len(headersPayload.Headers), p.Address, startHeight)
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

			response, err := server.SendRequest(p, msg)
			if err != nil {
				responses <- headerResponse{nil, err, p.Address}
				return
			}

			responseMsg := response
			if responseMsg.Type != MessageTypeHeaders {
				responses <- headerResponse{nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type), p.Address}
				return
			}

			var headersPayload HeadersPayload
			if err := responseMsg.ParsePayload(&headersPayload); err != nil {
				responses <- headerResponse{nil, fmt.Errorf("failed to parse headers response: %w", err), p.Address}
				return
			}

			// Validate each header structure
			for i, header := range headersPayload.Headers {
				if err := blockchain.ValidateHeaderStructure(&header); err != nil {
					responses <- headerResponse{nil, fmt.Errorf("invalid header %d from peer: %w", i, err), p.Address}
					return
				}
			}

			server.logf("Received %d valid headers from %s by hash", len(headersPayload.Headers), p.Address)
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

			response, err := server.SendRequest(p, msg)
			if err != nil {
				responses <- peerResponse{nil, err, p.Address}
				return
			}

			responseMsg := response
			if responseMsg.Type != MessageTypeBlocks {
				responses <- peerResponse{nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type), p.Address}
				return
			}

			var blocksPayload BlocksPayload
			if err := responseMsg.ParsePayload(&blocksPayload); err != nil {
				responses <- peerResponse{nil, fmt.Errorf("failed to parse blocks response: %w", err), p.Address}
				return
			}

			// Validate each block structure
			for i, block := range blocksPayload.Blocks {
				if err := blockchain.ValidateBlockStructure(block); err != nil {
					responses <- peerResponse{nil, fmt.Errorf("invalid block %d from peer: %w", i, err), p.Address}
					return
				}
			}

			server.logf("Received %d valid blocks from %s", len(blocksPayload.Blocks), p.Address)
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
func EvaluateChainHead(server *Server, peer *Peer, peerChainHead *consensus.ChainHeadPayload) error {
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

	// Create candidate chain using consensus manager
	candidate, err := server.consensusManager.CreateCandidateChain(peer.Address, headers, peerChainHead)
	if err != nil {
		return fmt.Errorf("failed to create candidate chain: %w", err)
	}

	// Start download in background
	go func() {
		if err := server.chainDownloader.DownloadCandidateChain(candidate); err != nil {
			server.logf("Candidate chain download failed: %v", err)
		}
	}()

	return nil
}

// ProcessBlock delegates to the node's ProcessBlock method via callback
// Returns a completion channel that will be closed when processing completes
// excludePeerAddr: if provided, this peer will be excluded from relay (used when block came from a peer)
func ProcessBlock(server *Server, block *blockchain.Block, excludePeerAddr ...string) <-chan struct{} {
	if server.blockProcessor == nil {
		// If no processor is set, return a completed channel
		complete := make(chan struct{})
		close(complete)
		return complete
	}

	return server.blockProcessor(block, excludePeerAddr...)
}

// GetCandidateBlockCount returns the number of candidate blocks waiting for parents (for testing)
func GetCandidateBlockCount(server *Server) int {
	// This would need to be implemented in the consensus manager if needed for testing
	// For now, return 0 as a placeholder
	return 0
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
	peers := s.GetConnectedPeers()
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
		blockHash := block.Header.GetHash()
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
	requesterAddr := "anonymous"
	if peer != nil {
		requesterAddr = peer.Address
	}
	s.logf("Requester %s requested chain head", requesterAddr)

	chain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get chain: %v", err)
		return
	}

	if len(chain.Blocks) == 0 {
		s.logf("No blocks in chain to share with %s", requesterAddr)
		return
	}

	// Get our chain head
	headBlock := chain.Blocks[len(chain.Blocks)-1]
	headHash := headBlock.Header.GetHash()

	headPayload := consensus.ChainHeadPayload{
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
		s.logf("Failed to send chain head to %s: %v", requesterAddr, err)
	} else {
		s.logf("Sent chain head (height %d) to %s", headBlock.Header.Height, requesterAddr)
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

	// Consistent lock ordering to prevent deadlocks
	// Always acquire peerConnectionsMu before peersMu
	s.peerConnectionsMu.Lock()
	s.peersMu.Lock()

	existingPeer, exists := s.peers[properPeerAddr]

	if exists {
		// Check if this is the same peer object (outgoing connection) or a different one (race condition)
		if existingPeer == peer {
			// Same peer object, just update it
			existingPeer.ID = handshake.NodeID
			existingPeer.Status = PeerConnected
			s.peersMu.Unlock()
			s.peerConnectionsMu.Unlock()
		} else {
			// Different peer objects - this is a race condition with bidirectional connections
			// We need to decide which connection to keep based on a deterministic rule
			keepExisting := s.shouldKeepExistingConnection(existingPeer, peer, handshake.NodeID)

			if keepExisting {
				s.logf("Duplicate connection detected for %s, keeping existing connection (existing: %v, new: %v)",
					properPeerAddr, existingPeer.IsOutgoing, peer.IsOutgoing)
				// Ensure existing peer is marked as properly connected
				if existingPeer.Status != PeerConnected {
					existingPeer.Status = PeerConnected
					existingPeer.ID = handshake.NodeID
				}
				s.peersMu.Unlock()
				s.peerConnectionsMu.Unlock()
				conn.Close()
				return
			} else {
				s.logf("Duplicate connection detected for %s, replacing existing connection (existing: %v, new: %v)",
					properPeerAddr, existingPeer.IsOutgoing, peer.IsOutgoing)

				// Close the old connection and replace with new one (locks already held)
				if oldConn, exists := s.peerConnections[existingPeer.ConnAddr]; exists {
					oldConn.Close()
					delete(s.peerConnections, existingPeer.ConnAddr)
				}
				if oldConn, exists := s.peerConnections[properPeerAddr]; exists && oldConn != conn {
					oldConn.Close()
				}
				s.peerConnections[properPeerAddr] = conn

				// Update the existing peer entry with new connection info
				existingPeer.ID = handshake.NodeID
				existingPeer.ConnAddr = conn.RemoteAddr().String()
				existingPeer.Status = PeerConnected
				existingPeer.IsOutgoing = peer.IsOutgoing
				existingPeer.LastSeen = time.Now() // Update last seen time
				s.peersMu.Unlock()
				s.peerConnectionsMu.Unlock()
			}
		}
	} else {
		// No existing connection - add this peer
		peer.ID = handshake.NodeID
		peer.Status = PeerConnected

		// Update the connection mapping to use the proper address (locks already held)
		if peer.Address != properPeerAddr {
			// Move the connection from ephemeral to proper address
			if conn, ok := s.peerConnections[peer.Address]; ok {
				delete(s.peerConnections, peer.Address)
				s.peerConnections[properPeerAddr] = conn
			}
		}

		// ConnAddr stays as the actual connection endpoint
		// Address becomes the listen address
		oldAddr := peer.Address
		peer.Address = properPeerAddr
		if peer.ConnAddr == oldAddr {
			// ConnAddr was same as Address, keep it as the connection endpoint
			// For incoming connections, ConnAddr is the ephemeral port they connected from
		}

		// Add to peer manager with proper address
		s.peers[properPeerAddr] = peer
		s.peersMu.Unlock()
		s.peerConnectionsMu.Unlock()
	}

	s.logf("Handshake completed with %s (node: %s, conn: %s, outgoing: %v)",
		properPeerAddr, handshake.NodeID, peer.ConnAddr, peer.IsOutgoing)
}

// ProcessChainHead handles chain head messages and initiates sync if needed
func (s *Server) ProcessChainHead(msg *Message, peer *Peer) {
	var headPayload consensus.ChainHeadPayload
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
			otherPeers := s.GetConnectedPeers()
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
	blockHash := header.GetHash()

	// Check if we already have this block header in our recent blocks (deduplication)
	s.recentBlocksMu.Lock()
	defer s.recentBlocksMu.Unlock()

	if addedTime, seen := s.recentBlocks[blockHash]; seen {
		if time.Now().Sub(addedTime) <= config.RecentBlocksTTL {
			s.logf("Ignoring duplicate block header: %x", blockHash[:8])
			return
		}
	}
	s.recentBlocks[blockHash] = time.Now()

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
	finalHash := finalHeader.GetHash()
	chainHead := &consensus.ChainHeadPayload{
		HeadHash:  base64.StdEncoding.EncodeToString(finalHash[:]),
		Height:    finalHeader.Height,
		TotalWork: finalHeader.TotalWork,
		Header:    finalHeader,
	}

	// Start candidate chain download using consensus manager
	candidate, err := s.consensusManager.CreateCandidateChain(peer.Address, headersPayload.Headers, chainHead)
	if err != nil {
		s.logf("Failed to create candidate chain from %s: %v", peer.Address, err)
		return
	}

	// Start download in background
	go func() {
		if err := s.chainDownloader.DownloadCandidateChain(candidate); err != nil {
			s.logf("Candidate chain download failed: %v", err)
		}
	}()
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
		blockHash := block.Header.GetHash()
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
	blockHash := blockPayload.Block.Header.GetHash()
	s.recentBlocksMu.Lock()
	defer s.recentBlocksMu.Unlock()

	if addedTime, seen := s.recentBlocks[blockHash]; seen {
		if time.Now().Sub(addedTime) <= config.RecentBlocksTTL {
			s.logf("Ignoring new block: %x because its in recentblocks", blockHash[:8])
			return
		}
	}
	s.recentBlocks[blockHash] = time.Now()

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
	// First, try to parse as direct transaction (for new relay)
	var tx *blockchain.Transaction
	if err := msg.ParsePayload(&tx); err != nil {
		// Try legacy format with wrapper
		var txPayload NewTxPayload
		if err := msg.ParsePayload(&txPayload); err != nil {
			s.logf("Failed to parse transaction payload: %v", err)
			return
		}
		tx = txPayload.Transaction
	}

	if tx == nil {
		s.logf("Received nil transaction from peer %s", peer.Address)
		return
	}

	txHash := tx.GetHash()

	// Check if we've recently seen this transaction (prevent loops)
	if s.IsRecentTransaction(txHash) {
		s.logf("Already seen transaction %x, skipping", txHash[:8])
		return
	}

	s.logf("Received new transaction %x from peer %s", txHash[:8], peer.Address)

	// Mark as seen to prevent loops
	s.MarkTransactionSeen(txHash)

	// Add to our mempool if we have a transaction processor
	if s.config.TransactionProcessor != nil {
		if err := s.config.TransactionProcessor(tx); err != nil {
			s.logf("Transaction %x rejected by mempool: %v", txHash[:8], err)
			return // Don't relay invalid transactions
		}
	}

	// Relay to other peers (excluding the sender)
	go func() { <-RelayTransaction(s, tx, peer.Address) }()
}

// ProcessSubmitBlock handles one-off block submissions from miners (no peer relationship required)
func (s *Server) ProcessSubmitBlock(msg *Message, conn net.Conn) {
	var submitPayload SubmitBlockPayload
	if err := msg.ParsePayload(&submitPayload); err != nil {
		s.logf("Failed to parse submit block payload: %v", err)
		return
	}

	blockHash := submitPayload.Block.Header.GetHash()
	remoteAddr := conn.RemoteAddr().String()
	s.logf("Received block submission %x from miner %s", blockHash[:8], remoteAddr)

	// Process the block through the same pipeline
	// (no excludePeerAddr since this isn't from a peer)
	go func() {
		<-ProcessBlock(s, submitPayload.Block)
	}()

	// Send simple acknowledgment back to miner
	response := map[string]interface{}{
		"status": "received",
		"block_hash": fmt.Sprintf("%x", blockHash[:8]),
	}

	ackMsg, err := NewMessage(MessageTypePong, response) // Reuse pong for simplicity
	if err == nil {
		s.sendMessage(conn, ackMsg)
	}
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
			blockHash := block.Header.GetHash()
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
				blockHash := block.Header.GetHash()
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
