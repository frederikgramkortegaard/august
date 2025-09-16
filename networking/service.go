package networking

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"time"

	"august/config"
	. "august/types"
)



// RelayBlock broadcasts a block to all connected peers, optionally excluding specific peers
// Returns a completion channel that will be closed when the relay operation completes
func (n *Network) RelayBlock(block *Block, excludePeerAddrs ...string) <-chan struct{} {
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
			n.recentBlocksMu.Lock()
			n.recentBlocks[blockHash] = time.Now()
			n.recentBlocksMu.Unlock()
		}

		if len(excludePeerAddrs) == 0 {
			fmt.Printf("Relaying block %x to all peers\n", blockHash[:8])
		} else {
			fmt.Printf("Relaying block %x to other peers (excluding %d peers)\n", blockHash[:8], len(excludePeerAddrs))
		}

		// Create the block payload
		blockPayload := NewBlockPayload{Block: block}
		msg, err := NewMessage(MessageTypeNewBlock, blockPayload)
		if err != nil {
			fmt.Printf("Failed to create relay message for block %x: %v\n", blockHash[:8], err)
			return
		}

		// Send to all connected peers except the excluded ones
		relayCount := 0
		peers := n.GetConnectedPeers()
		for _, peer := range peers {
			if !excludeMap[peer.Address] && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := n.SendNotification(peer.Address, msg); err != nil {
					fmt.Printf("Failed to relay block %x to peer %s: %v\n", blockHash[:8], peer.Address, err)
				} else {
					fmt.Printf("Relayed block %x to peer %s\n", blockHash[:8], peer.Address)
					relayCount++
				}
			}
		}

		fmt.Printf("Successfully relayed block %x to %d peers\n", blockHash[:8], relayCount)
	}()

	return complete
}

// RelayTransaction broadcasts a transaction to all connected peers, optionally excluding specific peers
// Returns a completion channel that will be closed when the relay operation completes
func (n *Network) RelayTransaction(tx *Transaction, excludePeerAddrs ...string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		txHash := tx.GetHash()

		// Mark this transaction as seen (for deduplication)
		n.MarkTransactionSeen(txHash)

		// Create map for quick exclusion lookup
		excludeMap := make(map[string]bool)
		for _, addr := range excludePeerAddrs {
			excludeMap[addr] = true
		}

		// Get list of connected peers
		peers := n.GetConnectedPeers()

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
				fmt.Printf("Failed to create transaction message: %v\n", err)
				continue
			}

			err = n.SendNotification(peer.Address, msg)
			if err != nil {
				fmt.Printf("Failed to relay transaction %x to peer %s: %v\n", txHash[:8], peer.Address, err)
			} else {
				relayCount++
			}
		}

		if relayCount > 0 {
			fmt.Printf("Relayed transaction %x to %d peers\n", txHash[:8], relayCount)
		}
	}()

	return complete
}

// RelayBlockHeader broadcasts a block header to all connected peers (headers-first), optionally excluding specific peers
// Returns a completion channel that will be closed when the relay operation completes
func (n *Network) RelayBlockHeader(header *BlockHeader, excludePeerAddrs ...string) <-chan struct{} {
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
			n.recentBlocksMu.Lock()
			n.recentBlocks[blockHash] = time.Now()
			n.recentBlocksMu.Unlock()
		}

		if len(excludePeerAddrs) == 0 {
			fmt.Printf("Relaying block header %x (height %d) to all peers\n", blockHash[:8], header.Height)
		} else {
			fmt.Printf("Relaying block header %x (height %d) to other peers (excluding %d peers)\n",
				blockHash[:8], header.Height, len(excludePeerAddrs))
		}

		// Create the header payload
		headerPayload := NewBlockHeaderPayload{Header: *header}
		msg, err := NewMessage(MessageTypeNewBlockHeader, headerPayload)
		if err != nil {
			fmt.Printf("Failed to create relay message for header %x: %v\n", blockHash[:8], err)
			return
		}

		// Send to all connected peers except the excluded ones
		relayCount := 0
		peers := n.GetConnectedPeers()
		for _, peer := range peers {
			if !excludeMap[peer.Address] && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := n.SendNotification(peer.Address, msg); err != nil {
					fmt.Printf("Failed to relay header %x to peer %s: %v\n", blockHash[:8], peer.Address, err)
				} else {
					fmt.Printf("Relayed header %x to peer %s\n", blockHash[:8], peer.Address)
					relayCount++
				}
			}
		}

		fmt.Printf("Successfully relayed header %x to %d peers\n", blockHash[:8], relayCount)
	}()

	return complete
}


// RequestChainHead requests the current chain head from a peer
func (n *Network) RequestChainHead(peer *Peer) (*ChainHeadPayload, error) {
	requestPayload := RequestChainHeadPayload{}

	msg, err := NewMessage(MessageTypeRequestChainHead, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain head request: %w", err)
	}

	response, err := n.SendRequest(peer.Address, msg)
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

	fmt.Printf("Received chain head from %s: height=%d, work=%s\n", peer.Address, headPayload.Height, headPayload.TotalWork)
	return &headPayload, nil
}

// RequestHeadersByHeight requests block headers in a range using smart peer selection
func (n *Network) RequestHeadersByHeight(startHeight uint64, count uint64) ([]BlockHeader, error) {
	// Use smart peer selection (handles getting connected peers internally)
	selectedPeers := n.selectPeersForRequest()
	if len(selectedPeers) == 0 {
		return nil, fmt.Errorf("no suitable peers available")
	}

	// Request from selected peers in parallel
	type headerResponse struct {
		headers []BlockHeader
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

			response, err := n.SendRequest(p.Address, msg)
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

			// Validate each header structure
			for i, header := range headersPayload.Headers {
				// TODO: Move validation function to avoid import cycle
				//if err := ValidateHeaderStructure(&header); err != nil {
				//	responses <- headerResponse{nil, fmt.Errorf("invalid header %d from peer: %w", i, err), p.Address}
				//	return
				//}
				_ = i
				_ = header
			}

			fmt.Printf("Received %d valid headers from %s starting at height %d\n", len(headersPayload.Headers), p.Address, startHeight)
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
			fmt.Printf("Header request failed from %s: %v\n", resp.peer, resp.err)
		}
	}

	if len(successfulResponses) == 0 {
		return nil, fmt.Errorf("all peers failed to provide headers starting at height %d", startHeight)
	}

	// For now, just return the first successful response
	// TODO: Add consensus verification later if needed
	fmt.Printf("Got headers from %d/%d peers, using first successful\n", len(successfulResponses), len(selectedPeers))
	return successfulResponses[0].headers, nil
}

// RequestHeadersByHash requests specific headers by their hashes using smart peer selection
func (n *Network) RequestHeadersByHash(blockHashes []string) ([]BlockHeader, error) {
	// Use smart peer selection (handles getting connected peers internally)
	selectedPeers := n.selectPeersForRequest()
	if len(selectedPeers) == 0 {
		return nil, fmt.Errorf("no suitable peers available")
	}

	// Request from selected peers in parallel
	type headerResponse struct {
		headers []BlockHeader
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

			response, err := n.SendRequest(p.Address, msg)
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

			// Validate each header structure
			for i, header := range headersPayload.Headers {
				// TODO: Move validation function to avoid import cycle
				//if err := ValidateHeaderStructure(&header); err != nil {
				//	responses <- headerResponse{nil, fmt.Errorf("invalid header %d from peer: %w", i, err), p.Address}
				//	return
				//}
				_ = i
				_ = header
			}

			fmt.Printf("Received %d valid headers from %s by hash\n", len(headersPayload.Headers), p.Address)
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
			fmt.Printf("Header request failed from %s: %v\n", resp.peer, resp.err)
		}
	}

	if len(successfulResponses) == 0 {
		return nil, fmt.Errorf("all peers failed to provide headers for requested hashes")
	}

	// For now, just return the first successful response
	// TODO : Add consensus verification later if needed
	// TODO : Handle dropout etc.
	fmt.Printf("Got headers from %d/%d peers, using first successful\n", len(successfulResponses), len(selectedPeers))
	return successfulResponses[0].headers, nil
}

// RequestBlocksByHash requests specific blocks by their hashes using Bitcoin-style peer selection
func (n *Network) RequestBlocksByHash(blockHashes []string) ([]*Block, error) {
	// Use smart peer selection (handles getting connected peers internally)
	selectedPeers := n.selectPeersForRequest()
	if len(selectedPeers) == 0 {
		return nil, fmt.Errorf("no suitable peers available")
	}

	// Request from selected peers in parallel
	type peerResponse struct {
		blocks []*Block
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

			response, err := n.SendRequest(p.Address, msg)
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

			// Block structure validation delegated to node processing layer
			for i, block := range blocksPayload.Blocks {
				_ = i
				_ = block
			}

			fmt.Printf("Received %d valid blocks from %s\n", len(blocksPayload.Blocks), p.Address)
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
			fmt.Printf("Failed to get blocks from %s: %v\n", resp.peer, resp.err)
		}
	}

	if len(successfulResponses) == 0 {
		return nil, fmt.Errorf("all peers failed to provide blocks")
	}

	// For now, just return the first successful response
	// TODO: Add consensus verification later if needed
	fmt.Printf("Got blocks from %d/%d peers, using first successful\n", len(successfulResponses), len(selectedPeers))
	return successfulResponses[0].blocks, nil
}

// EvaluateChainHead performs headers-first evaluation of a peer's chain
func (n *Network) EvaluateChainHead(peer *Peer, peerChainHead *ChainHeadPayload) error {
	// Get our current chain - TODO: This needs to be passed in or available through Network
	//ourChain, err := n.config.Store.GetChain()
	//if err != nil {
	//	return fmt.Errorf("failed to get our chain: %w", err)
	//}

	ourHeight := uint64(0)
	ourWork := "0"
	//if len(ourChain.Blocks) > 0 {
	//	ourHeight = ourChain.Blocks[len(ourChain.Blocks)-1].Header.Height
	//	ourWork = ourChain.Blocks[len(ourChain.Blocks)-1].Header.TotalWork
	//}

	fmt.Printf("Evaluating chain from %s: our height=%d work=%s, peer height=%d work=%s\n",
		peer.Address, ourHeight, ourWork, peerChainHead.Height, peerChainHead.TotalWork)

	// Quick work comparison - delegated to node processing layer

	// Phase 1: Download headers first from multiple peers (lightweight evaluation)
	fmt.Printf("Peer chain looks better, downloading headers from multiple peers (triggered by %s)\n", peer.Address)

	// Request headers using smart peer selection
	headers, err := n.RequestHeadersByHeight(1, peerChainHead.Height)
	if err != nil {
		return fmt.Errorf("failed to download headers from %s: %w", peer.Address, err)
	}

	// Header chain validation delegated to node processing layer

	fmt.Printf("Downloaded and validated %d headers from %s\n", len(headers), peer.Address)

	// Work calculation and comparison delegated to node processing layer
	finalHeaderWork := headers[len(headers)-1].TotalWork

	// Phase 2: Headers look good, start candidate chain download
	fmt.Printf("Headers validated and better, starting candidate chain download\n")

	// Create candidate chain using consensus manager - TODO: This needs to be available through Network
	//candidate, err := n.consensusManager.CreateCandidateChain(peer.Address, headers, peerChainHead)
	//if err != nil {
	//	return fmt.Errorf("failed to create candidate chain: %w", err)
	//}

	// Start download in background
	//go func() {
	//	if err := n.chainDownloader.DownloadCandidateChain(candidate); err != nil {
	//		fmt.Printf("Candidate chain download failed: %v\n", err)
	//	}
	//}()

	_ = finalHeaderWork

	return nil
}

// ProcessBlock delegates to the node's ProcessBlock method via callback
// Returns a completion channel that will be closed when processing completes
// excludePeerAddr: if provided, this peer will be excluded from relay (used when block came from a peer)
func (n *Network) ProcessBlock(block *Block, excludePeerAddr ...string) <-chan struct{} {
	// TODO: This needs to be available through Network or passed as callback
	//if n.blockProcessor == nil {
	//	// If no processor is set, return a completed channel
	//	complete := make(chan struct{})
	//	close(complete)
	//	return complete
	//}

	//return n.blockProcessor(block, excludePeerAddr...)

	// For now, return a completed channel
	complete := make(chan struct{})
	close(complete)
	return complete
}

// GetCandidateBlockCount returns the number of candidate blocks waiting for parents (for testing)
func (n *Network) GetCandidateBlockCount() int {
	// This would need to be implemented in the consensus manager if needed for testing
	// For now, return 0 as a placeholder
	return 0
}

// SendPongResponse sends a pong response to a ping
func (n *Network) SendPongResponse(conn net.Conn) {
	pongMsg, err := NewMessage(MessageTypePong, nil)
	if err == nil {
		n.sendMessage(conn, pongMsg)
	}
}

// SendPeerList sends a list of peers in response to a request
func (n *Network) SendPeerList(conn net.Conn, msg *Message, requesterAddr string) {
	fmt.Printf("Peer %s requested peer list\n", requesterAddr)

	// Get list of connected peers (excluding the requester)
	peers := n.GetConnectedPeers()
	var peerAddresses []string
	for _, p := range peers {
		if p.Address != requesterAddr && p.Status == PeerConnected {
			peerAddresses = append(peerAddresses, p.Address)
		}
	}

	fmt.Printf("Sharing %d peers with %s\n", len(peerAddresses), requesterAddr)

	sharePayload := SharePeersPayload{
		Peers: peerAddresses,
	}

	shareMsg, err := NewMessage(MessageTypeSharePeers, sharePayload)
	if err != nil {
		fmt.Printf("Failed to create share peers message: %v\n", err)
		return
	}

	// Set reply info if this is a response
	if msg.RequestID != "" {
		shareMsg.ReplyTo = msg.RequestID
	}

	n.sendMessage(conn, shareMsg)
}

// SendBlockResponse sends a block in response to a request
func (n *Network) SendBlockResponse(conn net.Conn, msg *Message, peer *Peer) {
	var payload RequestBlockPayload
	if err := msg.ParsePayload(&payload); err != nil {
		fmt.Printf("Failed to decode request block payload: %v\n", err)
		return
	}

	fmt.Printf("Peer %s requested block %s\n", peer.Address, payload.BlockHash)

	// TODO: This function needs access to blockchain storage which should be at Node level
	fmt.Printf("Block %s not found for peer %s (blockchain access not available at Network level)\n", payload.BlockHash, peer.Address)
}

// SendChainHeadResponse sends chain head information in response to a request
func (n *Network) SendChainHeadResponse(conn net.Conn, msg *Message, peer *Peer) {
	requesterAddr := "anonymous"
	if peer != nil {
		requesterAddr = peer.Address
	}
	fmt.Printf("Requester %s requested chain head\n", requesterAddr)

	// TODO: This function needs access to blockchain storage which should be at Node level
	fmt.Printf("No chain head available for %s (blockchain access not available at Network level)\n", requesterAddr)
}

// ProcessHandshake handles the business logic for processing a handshake
func (n *Network) ProcessHandshake(msg *Message, peer *Peer, conn net.Conn) {
	var handshake HandshakePayload
	if err := msg.ParsePayload(&handshake); err != nil {
		fmt.Printf("Failed to decode handshake payload from %s: %v\n", peer.Address, err)
		return
	}

	// Construct the proper peer address using IP from connection + ListenPort from handshake
	remoteAddr := conn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		fmt.Printf("Failed to parse remote address %s: %v\n", remoteAddr, err)
		return
	}

	properPeerAddr := net.JoinHostPort(host, handshake.ListenPort)
	fmt.Printf("Received handshake from %s (node %s), actual peer address: %s\n",
		peer.Address, handshake.NodeID, properPeerAddr)

	// Consistent lock ordering to prevent deadlocks
	// Always acquire peerConnectionsMu before peerManager.mu
	n.peerConnectionsMu.Lock()
	n.peersMu.Lock()

	existingPeer, exists := n.peers[properPeerAddr]

	if exists {
		// Check if this is the same peer object (outgoing connection) or a different one (race condition)
		if existingPeer == peer {
			// Same peer object, just update it
			existingPeer.ID = handshake.NodeID
			existingPeer.Status = PeerConnected
			s.peerManager.mu.Unlock()
			s.peerConnectionsMu.Unlock()
		} else {
			// Different peer objects - this is a race condition with bidirectional connections
			// We need to decide which connection to keep based on a deterministic rule
			keepExisting := s.shouldKeepExistingConnection(existingPeer, peer, handshake.NodeID)

			if keepExisting {
				fmt.Printf("Duplicate connection detected for %s, keeping existing connection (existing: %v, new: %v)",
					properPeerAddr, existingPeer.IsOutgoing, peer.IsOutgoing)
				// Ensure existing peer is marked as properly connected
				if existingPeer.Status != PeerConnected {
					existingPeer.Status = PeerConnected
					existingPeer.ID = handshake.NodeID
				}
				s.peerManager.mu.Unlock()
				s.peerConnectionsMu.Unlock()
				conn.Close()
				return
			} else {
				fmt.Printf("Duplicate connection detected for %s, replacing existing connection (existing: %v, new: %v)",
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
				s.peerManager.mu.Unlock()
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
		s.peerManager.peers[properPeerAddr] = peer
		s.peerManager.mu.Unlock()
		s.peerConnectionsMu.Unlock()
	}

	fmt.Printf("Handshake completed with %s (node: %s, conn: %s, outgoing: %v)",
		properPeerAddr, handshake.NodeID, peer.ConnAddr, peer.IsOutgoing)
}

// ProcessChainHead handles chain head messages and initiates sync if needed
func (n *Network) ProcessChainHead(msg *Message, peer *Peer) {
	var headPayload ChainHeadPayload
	if err := msg.ParsePayload(&headPayload); err != nil {
		fmt.Printf("Failed to decode chain head payload from %s: %v", peer.Address, err)
		return
	}

	fmt.Printf("Received chain head from %s: height=%d, hash=%s",
		peer.Address, headPayload.Height, headPayload.HeadHash[:16])

	// Compare with our chain
	ourChain, err := s.config.Store.GetChain()
	if err != nil {
		fmt.Printf("Failed to get our chain for comparison: %v", err)
		return
	}

	ourHeight := uint64(0)
	if len(ourChain.Blocks) > 0 {
		ourHeight = ourChain.Blocks[len(ourChain.Blocks)-1].Header.Height
	}

	// If peer is significantly ahead (more than 1 block), initiate IBD
	if headPayload.Height > ourHeight+1 {
		fmt.Printf("Peer %s is ahead by %d blocks, initiating IBD",
			peer.Address, headPayload.Height-ourHeight)

		// Start headers-first evaluation in background
		go func() {
			if err := EvaluateChainHead(s, peer, &headPayload); err != nil {
				fmt.Printf("Chain evaluation failed from %s: %v", peer.Address, err)
			}
		}()
	} else if headPayload.Height == ourHeight+1 {
		fmt.Printf("Peer %s has next block, requesting it", peer.Address)

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
				fmt.Printf("Failed to get next block from peers: %v", err)
				return
			}

			if len(blocks) == 0 {
				fmt.Printf("No next block returned from peers\n")
				return
			}

			// Process the single block
			<-ProcessBlock(s, blocks[0], peer.Address)
		}()
	}
}

// ProcessNewBlockHeader handles incoming block header announcements
func (n *Network) ProcessNewBlockHeader(msg *Message, peer *Peer) {
	var headerPayload NewBlockHeaderPayload
	if err := msg.ParsePayload(&headerPayload); err != nil {
		fmt.Printf("Failed to parse block header payload: %v\n", err)
		return
	}

	header := &headerPayload.Header
	blockHash := header.GetHash()

	// Check if we already have this block header in our recent blocks (deduplication)
	n.recentBlocksMu.Lock()
	defer n.recentBlocksMu.Unlock()

	if addedTime, seen := n.recentBlocks[blockHash]; seen {
		if time.Now().Sub(addedTime) <= n.recentBlocksTTL {
			fmt.Printf("Ignoring duplicate block header: %x\n", blockHash[:8])
			return
		}
	}
	n.recentBlocks[blockHash] = time.Now()

	fmt.Printf("Received new block header %x (height %d) from peer %s\n",
		blockHash[:8], header.Height, peer.Address)

	// Delegate to node for processing (which will handle evaluation and requests)
	go func() {
		if err := n.node.ProcessNewBlockHeader(header, peer.Address); err != nil {
			fmt.Printf("Failed to process block header %x: %v\n", blockHash[:8], err)
		}
	}()
}

// ProcessHeaders handles received headers and integrates them with headers-first sync
func (n *Network) ProcessHeaders(msg *Message, peer *Peer) {
	var headersPayload HeadersPayload
	if err := msg.ParsePayload(&headersPayload); err != nil {
		fmt.Printf("Failed to parse headers: %v", err)
		return
	}

	fmt.Printf("Received %d headers from %s", len(headersPayload.Headers), peer.Address)

	if len(headersPayload.Headers) == 0 {
		return
	}

	// Validate the header chain
	//if !blockchain.ValidateHeaderChain(headersPayload.Headers) {
	//	fmt.Printf("Invalid header chain from %s, rejecting", peer.Address)
	//	return
	//}

	// Check if these headers represent a better chain than ours
	ourChain, err := s.config.Store.GetChain()
	if err != nil {
		fmt.Printf("Failed to get our chain: %v", err)
		return
	}

	ourWork := "0"
	if len(ourChain.Blocks) > 0 {
		ourWork = ourChain.Blocks[len(ourChain.Blocks)-1].Header.TotalWork
	}

	finalHeaderWork := headersPayload.Headers[len(headersPayload.Headers)-1].TotalWork
	// Work comparison delegated to node processing layer

	fmt.Printf("Headers from %s represent better chain, starting candidate download", peer.Address)

	// Create a mock chain head payload from the final header
	finalHeader := headersPayload.Headers[len(headersPayload.Headers)-1]
	finalHash := finalHeader.GetHash()
	chainHead := &ChainHeadPayload{
		HeadHash:  base64.StdEncoding.EncodeToString(finalHash[:]),
		Height:    finalHeader.Height,
		TotalWork: finalHeader.TotalWork,
		Header:    finalHeader,
	}

	// Start candidate chain download using consensus manager
	candidate, err := s.consensusManager.CreateCandidateChain(peer.Address, headersPayload.Headers, chainHead)
	if err != nil {
		fmt.Printf("Failed to create candidate chain from %s: %v", peer.Address, err)
		return
	}

	// Start download in background
	go func() {
		if err := s.chainDownloader.DownloadCandidateChain(candidate); err != nil {
			fmt.Printf("Candidate chain download failed: %v", err)
		}
	}()
}

// ProcessBlocks handles received blocks
func (n *Network) ProcessBlocks(msg *Message, peer *Peer) {
	var blocksPayload BlocksPayload
	if err := msg.ParsePayload(&blocksPayload); err != nil {
		fmt.Printf("Failed to parse blocks: %v", err)
		return
	}

	fmt.Printf("Received %d blocks from %s", len(blocksPayload.Blocks), peer.Address)

	// Process each block sequentially
	for _, block := range blocksPayload.Blocks {
		blockHash := block.Header.GetHash()
		fmt.Printf("Processing batch block %x from %s", blockHash[:8], peer.Address)

		// Process block through the normal pipeline
		go func(b *Block) {
			<-ProcessBlock(s, b, peer.Address)
		}(block)
	}
}

// ProcessNewBlock handles incoming block announcements
func (n *Network) ProcessNewBlock(msg *Message, peer *Peer) {
	var blockPayload NewBlockPayload
	if err := msg.ParsePayload(&blockPayload); err != nil {
		fmt.Printf("Failed to parse block payload: %v\n", err)
		return
	}

	// Mitigate Broadcast Storm by keeping list of blocks to ignore
	blockHash := blockPayload.Block.Header.GetHash()
	n.recentBlocksMu.Lock()
	defer n.recentBlocksMu.Unlock()

	if addedTime, seen := n.recentBlocks[blockHash]; seen {
		if time.Now().Sub(addedTime) <= n.recentBlocksTTL {
			fmt.Printf("Ignoring new block: %x because its in recentblocks\n", blockHash[:8])
			return
		}
	}
	n.recentBlocks[blockHash] = time.Now()

	fmt.Printf("Received new block %x from peer %s\n", blockHash[:8], peer.Address)

	// Delegate to node for processing (exclude the sender from relay)
	go func() { <-n.node.ProcessBlock(blockPayload.Block, peer.Address) }()
}

// ProcessPong handles pong responses
func (n *Network) ProcessPong(peer *Peer) {
	fmt.Printf("Received pong from peer %s", peer.Address)
}

// ProcessSharedPeers handles received peer lists
func (n *Network) ProcessSharedPeers(msg *Message, peer *Peer) {
	var sharePayload SharePeersPayload
	if err := msg.ParsePayload(&sharePayload); err != nil {
		fmt.Printf("Failed to parse shared peers: %v", err)
		return
	}

	fmt.Printf("Received %d peers from %s", len(sharePayload.Peers), peer.Address)
	// The discovery component handles adding these peers
}

// ProcessNewTransaction handles incoming transaction announcements
func (n *Network) ProcessNewTransaction(msg *Message, peer *Peer) {
	// First, try to parse as direct transaction (for new relay)
	var tx *Transaction
	if err := msg.ParsePayload(&tx); err != nil {
		// Try legacy format with wrapper
		var txPayload NewTxPayload
		if err := msg.ParsePayload(&txPayload); err != nil {
			fmt.Printf("Failed to parse transaction payload: %v", err)
			return
		}
		tx = txPayload.Transaction
	}

	if tx == nil {
		fmt.Printf("Received nil transaction from peer %s", peer.Address)
		return
	}

	txHash := tx.GetHash()

	// Check if we've recently seen this transaction (prevent loops)
	if n.IsRecentTransaction(txHash) {
		fmt.Printf("Already seen transaction %x, skipping\n", txHash[:8])
		return
	}

	fmt.Printf("Received new transaction %x from peer %s\n", txHash[:8], peer.Address)

	// Mark as seen to prevent loops
	n.MarkTransactionSeen(txHash)

	// Delegate to node for processing (exclude the sender from relay)
	go func() {
		if err := n.node.ProcessTransaction(tx, peer.Address); err != nil {
			fmt.Printf("Transaction %x rejected: %v\n", txHash[:8], err)
		}
	}()
}

// ProcessSubmitBlock handles one-off block submissions from miners (no peer relationship required)
func (n *Network) ProcessSubmitBlock(msg *Message, conn net.Conn) {
	var submitPayload SubmitBlockPayload
	if err := msg.ParsePayload(&submitPayload); err != nil {
		fmt.Printf("Failed to parse submit block payload: %v", err)
		return
	}

	blockHash := submitPayload.Block.Header.GetHash()
	remoteAddr := conn.RemoteAddr().String()
	fmt.Printf("Received block submission %x from miner %s", blockHash[:8], remoteAddr)

	// Process the block through the same pipeline
	// (no excludePeerAddr since this isn't from a peer)
	go func() {
		<-ProcessBlock(s, submitPayload.Block)
	}()

	// Send simple acknowledgment back to miner
	response := map[string]interface{}{
		"status":     "received",
		"block_hash": fmt.Sprintf("%x", blockHash[:8]),
	}

	ackMsg, err := NewMessage(MessageTypePong, response) // Reuse pong for simplicity
	if err == nil {
		s.sendMessage(conn, ackMsg)
	}
}

// SendHeadersResponse sends headers in response to a request
func (n *Network) SendHeadersResponse(conn net.Conn, msg *Message, peer *Peer) {
	var payload RequestHeadersPayload
	if err := msg.ParsePayload(&payload); err != nil {
		fmt.Printf("Failed to decode request headers payload: %v", err)
		return
	}

	chain, err := s.config.Store.GetChain()
	if err != nil {
		fmt.Printf("Failed to get chain: %v", err)
		return
	}

	var headers []BlockHeader

	if len(payload.Hashes) > 0 {
		// Hash-based request
		fmt.Printf("Peer %s requested %d specific headers by hash", peer.Address, len(payload.Hashes))

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
		fmt.Printf("Peer %s requested %d headers starting from height %d",
			peer.Address, payload.Count, payload.StartHeight)

		for _, block := range chain.Blocks {
			if block.Header.Height >= payload.StartHeight &&
				block.Header.Height < payload.StartHeight+payload.Count {
				headers = append(headers, block.Header)
			}
		}
	}

	fmt.Printf("Sending %d headers to %s", len(headers), peer.Address)

	response, err := NewMessage(MessageTypeHeaders, HeadersPayload{Headers: headers})
	if err != nil {
		fmt.Printf("Failed to create headers response: %v", err)
		return
	}

	// Set reply info to correlate with request
	response.ReplyTo = msg.RequestID

	if err := s.sendMessage(conn, response); err != nil {
		fmt.Printf("Failed to send headers to %s: %v", peer.Address, err)
	}
}

// SendBlocksResponse sends blocks in response to a request
func (n *Network) SendBlocksResponse(conn net.Conn, msg *Message, peer *Peer) {
	var payload interface{}

	// Determine request type
	if msg.Type == MessageTypeRequestBlocks {
		var reqPayload RequestBlocksPayload
		if err := msg.ParsePayload(&reqPayload); err != nil {
			fmt.Printf("Failed to decode request blocks payload: %v", err)
			return
		}
		payload = reqPayload
	}

	chain, err := s.config.Store.GetChain()
	if err != nil {
		fmt.Printf("Failed to get chain: %v", err)
		return
	}

	var blocks []*Block

	switch p := payload.(type) {
	case RequestBlocksPayload:
		if len(p.Hashes) > 0 {
			// Hash-based request
			fmt.Printf("Peer %s requested %d specific blocks by hash",
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
			fmt.Printf("Peer %s requested %d blocks starting from height %d",
				peer.Address, p.Count, p.StartHeight)

			for _, block := range chain.Blocks {
				if block.Header.Height >= p.StartHeight &&
					block.Header.Height < p.StartHeight+p.Count {
					blocks = append(blocks, block)
				}
			}
		}
	}

	fmt.Printf("Sending %d blocks to %s", len(blocks), peer.Address)

	response, err := NewMessage(MessageTypeBlocks, BlocksPayload{Blocks: blocks})
	if err != nil {
		fmt.Printf("Failed to create blocks response: %v", err)
		return
	}

	// Set reply info to correlate with request
	response.ReplyTo = msg.RequestID

	if err := s.sendMessage(conn, response); err != nil {
		fmt.Printf("Failed to send blocks to %s: %v", peer.Address, err)
	}
}
