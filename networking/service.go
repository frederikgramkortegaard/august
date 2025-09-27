package networking

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"august/blockchain"
	"august/config"
)

// selectPeersForRequest implements Bitcoin-style peer selection with randomization and limits
func selectPeersForRequest(server *Server) []*Peer {
	// Get all connected peers
	availablePeers := server.GetConnectedPeers()
	if len(availablePeers) == 0 {
		return nil
	}

	// Get config from server
	// Use constants from config package

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

	log.Printf(server.config.NodeID+"\t"+"Selected %d peers for request from %d available (randomized: %v)",
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
			log.Printf(server.config.NodeID+"\t"+"Relaying block %s to all peers", blockHash.String())
		} else {
			log.Printf(server.config.NodeID+"\t"+"Relaying block %s to other peers (excluding %d peers)", blockHash.String(), len(excludePeerAddrs))
		}

		// Create the block payload
		blockPayload := NewBlockPayload{Block: block}
		msg, err := NewMessage(MessageTypeNewBlock, blockPayload)
		if err != nil {
			log.Printf(server.config.NodeID+"\t"+"Failed to create relay message for block %s: %v", blockHash.String(), err)
			return
		}

		// Send to all connected peers except the excluded ones
		relayCount := 0
		for _, peer := range server.GetConnectedPeers() {
			if !excludeMap[peer.Address] && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := server.SendNotification(peer, msg); err != nil {
					log.Printf(server.config.NodeID+"\t"+"Failed to relay block %s to peer %s: %v", blockHash.String(), peer.Address, err)
				} else {
					log.Printf(server.config.NodeID+"\t"+"Relayed block %s to peer %s", blockHash.String(), peer.Address)
					relayCount++
				}
			}
		}

		log.Printf(server.config.NodeID+"\t"+"Successfully relayed block %s to %d peers", blockHash.String(), relayCount)
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
				log.Printf(server.config.NodeID+"\t"+"Failed to create transaction message: %v", err)
				continue
			}

			err = server.SendNotification(peer, msg)
			if err != nil {
				log.Printf(server.config.NodeID+"\t"+"Failed to relay transaction %s to peer %s: %v", txHash.String(), peer.Address, err)
			} else {
				relayCount++
			}
		}

		if relayCount > 0 {
			log.Printf(server.config.NodeID+"\t"+"Relayed transaction %s to %d peers", txHash.String(), relayCount)
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
			log.Printf(server.config.NodeID+"\t"+"Relaying block header %s (height %d) to all peers", blockHash.String(), header.Height)
		} else {
			log.Printf(server.config.NodeID+"\t"+"Relaying block header %x (height %d) to other peers (excluding %d peers)",
				blockHash[:8], header.Height, len(excludePeerAddrs))
		}

		// Create the header payload
		headerPayload := NewBlockHeaderPayload{Header: *header}
		msg, err := NewMessage(MessageTypeNewBlockHeader, headerPayload)
		if err != nil {
			log.Printf(server.config.NodeID+"\t"+"Failed to create relay message for header %s: %v", blockHash.String(), err)
			return
		}

		// Send to all connected peers except the excluded ones
		relayCount := 0
		for _, peer := range server.GetConnectedPeers() {
			if !excludeMap[peer.Address] && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := server.SendNotification(peer, msg); err != nil {
					log.Printf(server.config.NodeID+"\t"+"Failed to relay header %s to peer %s: %v", blockHash.String(), peer.Address, err)
				} else {
					log.Printf(server.config.NodeID+"\t"+"Relayed header %s to peer %s", blockHash.String(), peer.Address)
					relayCount++
				}
			}
		}

		log.Printf(server.config.NodeID+"\t"+"Successfully relayed header %s to %d peers", blockHash.String(), relayCount)
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

			log.Printf(server.config.NodeID+"\t"+"Received %d peers from %s", len(sharePayload.Peers), p.Address)
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
			log.Printf(server.config.NodeID+"\t"+"Peer discovery failed from %s: %v", resp.peer, resp.err)
		}
	}

	log.Printf(server.config.NodeID+"\t"+"Discovered %d unique peers from %d sources", len(allDiscoveredPeers), len(selectedPeers))
	return allDiscoveredPeers, nil
}

// RequestChainHead requests the current chain head from a peer
func RequestChainHead(server *Server, peer *Peer) (*ChainHeadPayload, error) {
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

	var headPayload ChainHeadPayload
	if err := responseMsg.ParsePayload(&headPayload); err != nil {
		return nil, fmt.Errorf("failed to parse chain head response: %w", err)
	}

	log.Printf(server.config.NodeID+"\t"+"Received chain head from %s: height=%d, work=%s", peer.Address, headPayload.Height, headPayload.TotalWork)
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

			log.Printf(server.config.NodeID+"\t"+"Received %d valid headers from %s starting at height %d", len(headersPayload.Headers), p.Address, startHeight)
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
			log.Printf(server.config.NodeID+"\t"+"Header request failed from %s: %v", resp.peer, resp.err)
		}
	}

	if len(successfulResponses) == 0 {
		return nil, fmt.Errorf("all peers failed to provide headers starting at height %d", startHeight)
	}

	// For now, just return the first successful response
	// TODO: Add consensus verification later if needed
	log.Printf(server.config.NodeID+"\t"+"Got headers from %d/%d peers, using first successful", len(successfulResponses), len(selectedPeers))
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

			log.Printf(server.config.NodeID+"\t"+"Received %d valid headers from %s by hash", len(headersPayload.Headers), p.Address)
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
			log.Printf(server.config.NodeID+"\t"+"Header request failed from %s: %v", resp.peer, resp.err)
		}
	}

	if len(successfulResponses) == 0 {
		return nil, fmt.Errorf("all peers failed to provide headers for requested hashes")
	}

	// For now, just return the first successful response
	// TODO : Add consensus verification later if needed
	// TODO : Handle dropout etc.
	log.Printf(server.config.NodeID+"\t"+"Got headers from %d/%d peers, using first successful", len(successfulResponses), len(selectedPeers))
	return successfulResponses[0].headers, nil
}

// RequestBlocksByHash requests specific blocks by their hashes using Bitcoin-style peer selection
func (s *Server) RequestBlocksByHash(blockHashes []string) <-chan []*blockchain.Block {
	resultChan := make(chan []*blockchain.Block, 1)

	go func() {
		defer close(resultChan)

		// Use smart peer selection (handles getting connected peers internally)
		selectedPeers := selectPeersForRequest(s)
		if len(selectedPeers) == 0 {
			log.Printf(s.config.NodeID + "\t" + "No suitable peers available for block request")
			resultChan <- nil
			return
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

				response, err := s.SendRequest(p, msg)
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

				log.Printf(s.config.NodeID+"\t"+"Received %d valid blocks from %s", len(blocksPayload.Blocks), p.Address)
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
				log.Printf(s.config.NodeID+"\t"+"Failed to get blocks from %s: %v", resp.peer, resp.err)
			}
		}

		if len(successfulResponses) == 0 {
			log.Printf(s.config.NodeID + "\t" + "All peers failed to provide blocks")
			resultChan <- nil
			return
		}

		// For now, just return the first successful response
		// TODO: Add consensus verification later if needed
		log.Printf(s.config.NodeID+"\t"+"Got blocks from %d/%d peers, using first successful", len(successfulResponses), len(selectedPeers))
		resultChan <- successfulResponses[0].blocks
	}()

	return resultChan
}

// EvaluateChainHead forwards peer chain head to node for evaluation
func EvaluateChainHead(server *Server, peer *Peer, peerChainHead *ChainHeadPayload) error {
	// Forward the header to the node for consensus evaluation
	return server.node.ProcessHeader(&peerChainHead.Header, peer.Address)
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
	log.Printf(s.config.NodeID+"\t"+"Peer %s requested peer list", requesterAddr)

	// Get list of connected peers (excluding the requester)
	peers := s.GetConnectedPeers()
	var peerAddresses []string
	for _, p := range peers {
		if p.Address != requesterAddr && p.Status == PeerConnected {
			peerAddresses = append(peerAddresses, p.Address)
		}
	}

	log.Printf(s.config.NodeID+"\t"+"Sharing %d peers with %s", len(peerAddresses), requesterAddr)

	sharePayload := SharePeersPayload{
		Peers: peerAddresses,
	}

	shareMsg, err := NewMessage(MessageTypeSharePeers, sharePayload)
	if err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to create share peers message: %v", err)
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
		log.Printf(s.config.NodeID+"\t"+"Failed to decode request block payload: %v", err)
		return
	}

	log.Printf(s.config.NodeID+"\t"+"Peer %s requested block %s", peer.Address, payload.BlockHash)

	// Get the block from our chain store
	currentChain := s.node.GetCurrentChain()
	if currentChain == nil {
		log.Printf(s.config.NodeID + "\t" + "Failed to get chain: GetCurrentChain was nil")
		return
	}

	// Find the requested block
	var foundBlock *blockchain.Block

	// Convert string hash to Hash32
	blockHash, err := StringToHash32(payload.BlockHash)
	if err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to decode block hash: %v", err)
		return
	}

	if b := s.node.GetBlock(blockHash); b != nil {
		foundBlock = b
	} else {
		log.Printf(s.config.NodeID+"\t"+"Block %s not found for peer %s", payload.BlockHash, peer.Address)
		return
	}

	// Create response message with the block
	response, err := NewMessage(MessageTypeNewBlock, NewBlockPayload{Block: foundBlock})
	if err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to create block response: %v", err)
		return
	}

	// Set reply info to correlate with request
	response.ReplyTo = msg.RequestID

	if err := s.sendMessage(conn, response); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to send block to %s: %v", peer.Address, err)
	} else {
		log.Printf(s.config.NodeID+"\t"+"Sent block %s to peer %s", payload.BlockHash, peer.Address)
	}
}

// SendChainHeadResponse sends chain head information in response to a request
func (s *Server) SendChainHeadResponse(conn net.Conn, msg *Message, peer *Peer) {
	requesterAddr := "anonymous"
	if peer != nil {
		requesterAddr = peer.Address
	}
	log.Printf(s.config.NodeID+"\t"+"Requester %s requested chain head", requesterAddr)

	// Get the block from our chain store
	currentChain := s.node.GetCurrentChain()
	if currentChain == nil {
		log.Printf(s.config.NodeID + "\t" + "Failed to get chain: GetCurrentChain was nil")
		return
	}

	// Get our chain head
	headBlock := currentChain
	headHash := headBlock.Header.GetHash()

	headPayload := ChainHeadPayload{
		HeadHash:  Hash32ToString(headHash),
		Height:    headBlock.Header.Height,
		TotalWork: headBlock.Header.TotalWork,
		Header:    headBlock.Header,
	}

	response, err := NewMessage(MessageTypeChainHead, headPayload)
	if err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to create chain head response: %v", err)
		return
	}

	// Set reply info to correlate with request
	if msg.RequestID != "" {
		response.ReplyTo = msg.RequestID
	}

	if err := s.sendMessage(conn, response); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to send chain head to %s: %v", requesterAddr, err)
	} else {
		log.Printf(s.config.NodeID+"\t"+"Sent chain head (height %d) to %s", headBlock.Header.Height, requesterAddr)
	}
}

// ProcessHandshake handles the business logic for processing a handshake
func (s *Server) ProcessHandshake(msg *Message, peer *Peer, conn net.Conn) {
	var handshake HandshakePayload
	if err := msg.ParsePayload(&handshake); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to decode handshake payload from %s: %v", peer.Address, err)
		return
	}

	// Construct the proper peer address using IP from connection + ListenPort from handshake
	remoteAddr := conn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to parse remote address %s: %v", remoteAddr, err)
		return
	}

	properPeerAddr := net.JoinHostPort(host, handshake.ListenPort)
	log.Printf(s.config.NodeID+"\t"+"Received handshake from %s (node %s), actual peer address: %s",
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
				log.Printf(s.config.NodeID+"\t"+"Duplicate connection detected for %s, keeping existing connection (existing: %v, new: %v)",
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
				log.Printf(s.config.NodeID+"\t"+"Duplicate connection detected for %s, replacing existing connection (existing: %v, new: %v)",
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

	log.Printf(s.config.NodeID+"\t"+"Handshake completed with %s (node: %s, conn: %s, outgoing: %v)",
		properPeerAddr, handshake.NodeID, peer.ConnAddr, peer.IsOutgoing)
}

// ProcessChainHead handles chain head messages by forwarding to node
func (s *Server) ProcessChainHead(msg *Message, peer *Peer) {
	var headPayload ChainHeadPayload
	if err := msg.ParsePayload(&headPayload); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to decode chain head payload from %s: %v", peer.Address, err)
		return
	}

	log.Printf(s.config.NodeID+"\t"+"Received chain head from %s: height=%d, hash=%s",
		peer.Address, headPayload.Height, headPayload.HeadHash[:16])

	// Check if this is the same as our current chain head

	currentChain := s.node.GetCurrentChain()
	if currentChain == nil {
		log.Printf(s.config.NodeID + "\t" + "Failed to get chain: GetCurrentChain was nil")
		return
	}

	if headPayload.Height == currentChain.Header.Height && headPayload.HeadHash == Hash32ToString(currentChain.Header.GetHash()) {
		log.Printf(s.config.NodeID+"\t"+"Ignoring chain head from %s: same as our chain head", peer.Address)
		return
	}
	// Forward to node for consensus evaluation
	if err := s.node.ProcessHeader(&headPayload.Header, peer.Address); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Node rejected chain head from %s: %v", peer.Address, err)
	}
}

// ProcessNewBlockHeader parses and forwards block headers to the node
func (s *Server) ProcessNewBlockHeader(msg *Message, peer *Peer) {
	var headerPayload NewBlockHeaderPayload
	if err := msg.ParsePayload(&headerPayload); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to parse block header payload: %v", err)
		return
	}

	header := &headerPayload.Header
	blockHash := header.GetHash()

	// Check for recent duplicate headers to mitigate broadcast storms
	s.recentHeadersMu.Lock()
	if addedTime, seen := s.recentHeaders[blockHash]; seen {
		if time.Now().Sub(addedTime) <= config.RecentHeadersTTL {
			s.recentHeadersMu.Unlock()
			log.Printf(s.config.NodeID+"\t"+"Ignoring duplicate block header: %s", blockHash.String())
			return
		}
	}
	s.recentHeaders[blockHash] = time.Now()
	s.recentHeadersMu.Unlock()

	log.Printf(s.config.NodeID+"\t"+"Received new block header %s (height %d) from peer %s",
		blockHash.String(), header.Height, peer.Address)

	// Forward to node for processing (all business logic handled there)
	if err := s.node.ProcessHeader(header, peer.Address); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to process header %s: %v", blockHash.String(), err)
	}

	// Always relay the header to other peers (except sender)
	go func() { <-RelayBlockHeader(s, header, peer.Address) }()
}

// ProcessPong handles pong responses
func (s *Server) ProcessPong(peer *Peer) {
	log.Printf(s.config.NodeID+"\t"+"Received pong from peer %s", peer.Address)
}

// ProcessSharedPeers handles received peer lists
func (s *Server) ProcessSharedPeers(msg *Message, peer *Peer) {
	var sharePayload SharePeersPayload
	if err := msg.ParsePayload(&sharePayload); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to parse shared peers: %v", err)
		return
	}

	log.Printf(s.config.NodeID+"\t"+"Received %d peers from %s", len(sharePayload.Peers), peer.Address)
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
			log.Printf(s.config.NodeID+"\t"+"Failed to parse transaction payload: %v", err)
			return
		}
		tx = txPayload.Transaction
	}

	if tx == nil {
		log.Printf(s.config.NodeID+"\t"+"Received nil transaction from peer %s", peer.Address)
		return
	}

	txHash := tx.GetHash()

	// Check if we've recently seen this transaction (prevent loops)
	if s.IsRecentTransaction(txHash) {
		log.Printf(s.config.NodeID+"\t"+"Already seen transaction %x, skipping", txHash[:8])
		return
	}

	log.Printf(s.config.NodeID+"\t"+"Received new transaction %x from peer %s", txHash[:8], peer.Address)

	// Mark as seen to prevent loops
	s.MarkTransactionSeen(txHash)

	// Forward to node for processing
	if err := s.node.ProcessTransaction(tx); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Transaction %x rejected: %v", txHash[:8], err)
		return // Don't relay invalid transactions
	}

	// Relay to other peers (excluding the sender)
	go func() { <-RelayTransaction(s, tx, peer.Address) }()
}

// SendHeadersResponse sends headers in response to a request
func (s *Server) SendHeadersResponse(conn net.Conn, msg *Message, peer *Peer) {
	var payload RequestHeadersPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to decode request headers payload: %v", err)
		return
	}

	var headers []blockchain.BlockHeader

	if len(payload.Hashes) > 0 {
		// Hash-based request
		log.Printf(s.config.NodeID+"\t"+"Peer %s requested %d specific headers by hash", peer.Address, len(payload.Hashes))

		for _, hashStr := range payload.Hashes {
			hash, err := StringToHash32(hashStr)
			if err != nil {
				log.Printf(s.config.NodeID+"\t"+"Invalid hash %s: %v", hashStr, err)
				continue
			}

			if header := s.node.GetHeader(hash); header != nil {
				headers = append(headers, *header)
			}
		}
	} else {
		// Height-based request
		log.Printf(s.config.NodeID+"\t"+"Peer %s requested %d headers starting from height %d",
			peer.Address, payload.Count, payload.StartHeight)

		for height := payload.StartHeight; height < payload.StartHeight+payload.Count; height++ {
			if block := s.node.GetBlockAtHeight(height); block != nil {
				headers = append(headers, block.Header)
			}
		}
	}

	log.Printf(s.config.NodeID+"\t"+"Sending %d headers to %s", len(headers), peer.Address)

	response, err := NewMessage(MessageTypeHeaders, HeadersPayload{Headers: headers})
	if err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to create headers response: %v", err)
		return
	}

	// Set reply info to correlate with request
	response.ReplyTo = msg.RequestID

	if err := s.sendMessage(conn, response); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to send headers to %s: %v", peer.Address, err)
	}
}

// SendBlocksResponse sends blocks in response to a request
func (s *Server) SendBlocksResponse(conn net.Conn, msg *Message, peer *Peer) {
	var payload interface{}

	// Determine request type
	if msg.Type == MessageTypeRequestBlocks {
		var reqPayload RequestBlocksPayload
		if err := msg.ParsePayload(&reqPayload); err != nil {
			log.Printf(s.config.NodeID+"\t"+"Failed to decode request blocks payload: %v", err)
			return
		}
		payload = reqPayload
	}

	var blocks []*blockchain.Block

	switch p := payload.(type) {
	case RequestBlocksPayload:
		if len(p.Hashes) > 0 {
			// Hash-based request
			log.Printf(s.config.NodeID+"\t"+"Peer %s requested %d specific blocks by hash",
				peer.Address, len(p.Hashes))

			for _, hashStr := range p.Hashes {
				hash, err := StringToHash32(hashStr)
				if err != nil {
					log.Printf(s.config.NodeID+"\t"+"Invalid hash %s: %v", hashStr, err)
					continue
				}

				if block := s.node.GetBlock(hash); block != nil {
					blocks = append(blocks, block)
				}
			}
		} else {
			// Height-based request
			log.Printf(s.config.NodeID+"\t"+"Peer %s requested %d blocks starting from height %d",
				peer.Address, p.Count, p.StartHeight)

			for height := p.StartHeight; height < p.StartHeight+p.Count; height++ {
				if block := s.node.GetBlockAtHeight(height); block != nil {
					blocks = append(blocks, block)
				}
			}
		}
	}

	log.Printf(s.config.NodeID+"\t"+"Sending %d blocks to %s", len(blocks), peer.Address)

	response, err := NewMessage(MessageTypeBlocks, BlocksPayload{Blocks: blocks})
	if err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to create blocks response: %v", err)
		return
	}

	// Set reply info to correlate with request
	response.ReplyTo = msg.RequestID

	if err := s.sendMessage(conn, response); err != nil {
		log.Printf(s.config.NodeID+"\t"+"Failed to send blocks to %s: %v", peer.Address, err)
	}
}
