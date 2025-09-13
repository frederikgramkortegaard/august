package networking

import (
	"encoding/base64"
	"net"
	"strings"
	"time"

	"august/blockchain"
)

// ProcessMessage handles different types of network messages
func ProcessMessage(server *Server, msg *Message, peer *Peer, conn net.Conn) {
	// First check if this is a response to a pending request
	if handled := server.reqRespClient.HandleResponse(msg); handled {
		server.logf("Delivered response for request %s", msg.ReplyTo)
		return
	}

	switch msg.Type {
	case MessageTypeHandshake:
		handleHandshake(server, msg, peer, conn)
	case MessageTypeNewBlockHeader:
		handleNewBlockHeader(server, msg, peer)
	case MessageTypeNewBlock:
		handleNewBlock(server, msg, peer)
	case MessageTypePing:
		handlePing(server, conn)
	case MessageTypePong:
		handlePong(server, peer)
	case MessageTypeRequestPeers:
		handleRequestPeers(server, msg, peer, conn)
	case MessageTypeSharePeers:
		handleSharePeers(server, msg, peer)
	case MessageTypeRequestBlock:
		handleRequestBlock(server, msg, peer, conn)
	case MessageTypeNewTx:
		handleNewTransaction(server, msg, peer)
	case MessageTypeRequestChainHead:
		handleRequestChainHead(server, msg, peer, conn)
	case MessageTypeChainHead:
		handleChainHead(server, msg, peer)
	case MessageTypeRequestHeaders:
		handleRequestHeaders(server, msg, peer, conn)
	case MessageTypeHeaders:
		handleHeaders(server, msg, peer)
	case MessageTypeRequestBlocks:
		handleRequestBlocks(server, msg, peer, conn)
	case MessageTypeBlocks:
		handleBlocks(server, msg, peer)
	default:
		server.logf("Unknown message type: %s", msg.Type)
	}
}

// handleHandshake processes incoming handshake messages
func handleHandshake(server *Server, msg *Message, peer *Peer, conn net.Conn) {
	var handshake HandshakePayload
	if err := msg.ParsePayload(&handshake); err != nil {
		server.logf("Failed to parse handshake: %v", err)
		return
	}

	// Construct the proper peer address using IP from connection + ListenPort from handshake
	remoteAddr := conn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		server.logf("Failed to parse remote address %s: %v", remoteAddr, err)
		return
	}

	properPeerAddr := net.JoinHostPort(host, handshake.ListenPort)
	server.logf("Handshake debug: current=%s, proper=%s, listenPort=%s", peer.Address, properPeerAddr, handshake.ListenPort)

	// Check for duplicate connections (bidirectional connection detection)
	server.peerManager.mu.Lock()
	existingPeer, exists := server.peerManager.peers[properPeerAddr]
	if exists && existingPeer != peer {
		// We have a duplicate connection!
		// Determine which connection to keep based on address comparison
		localListenAddr := net.JoinHostPort(host, server.config.Port) // Use same IP, our port

		server.logf("Duplicate connection detected: local=%s, remote=%s", localListenAddr, properPeerAddr)

		if strings.Compare(localListenAddr, properPeerAddr) < 0 {
			// Our address is "smaller", keep incoming connection (this one)
			server.logf("Keeping incoming connection from %s (lower address)", properPeerAddr)

			// Close the existing outgoing connection
			existingPeer.Status = PeerDisconnected
			// Remove old peer and replace with this one
			delete(server.peerManager.peers, properPeerAddr)
		} else {
			// Remote address is "smaller", keep existing outgoing connection
			server.logf("Rejecting incoming connection from %s (keeping outgoing)", properPeerAddr)
			server.peerManager.mu.Unlock()

			// Close this connection and return
			conn.Close()
			peer.Status = PeerDisconnected
			return
		}
	}

	// Update peer address if different
	if peer.Address != properPeerAddr {
		server.logf("Updating peer address from %s to %s", peer.Address, properPeerAddr)

		// Remove from old address and add to new address
		delete(server.peerManager.peers, peer.Address)

		// Update the peer connection mapping
		server.peerConnectionsMu.Lock()
		delete(server.peerConnections, peer.Address)
		server.peerConnections[properPeerAddr] = conn
		server.peerConnectionsMu.Unlock()

		// Update peer address
		peer.Address = properPeerAddr
	}

	// Register the peer with proper address
	peer.ID = handshake.NodeID
	peer.Status = PeerConnected
	server.peerManager.peers[properPeerAddr] = peer
	server.peerManager.mu.Unlock()

	server.logf("Received handshake from %s (height: %d)", handshake.NodeID, handshake.ChainHeight)
}

// handleNewBlockHeader processes incoming block header announcements (headers-first)
func handleNewBlockHeader(server *Server, msg *Message, peer *Peer) {
	var headerPayload NewBlockHeaderPayload
	if err := msg.ParsePayload(&headerPayload); err != nil {
		server.logf("Failed to parse block header payload: %v", err)
		return
	}

	header := &headerPayload.Header
	blockHash := blockchain.HashBlockHeader(header)

	// Check if we already have this block header in our recent blocks (deduplication)
	server.recentBlocksMu.Lock()
	if addedTime, seen := server.recentBlocks[blockHash]; seen {
		if time.Now().Sub(addedTime) <= server.recentBlocksTTL {
			server.recentBlocksMu.Unlock()
			server.logf("Ignoring duplicate block header: %x", blockHash[:8])
			return
		}
	}
	server.recentBlocks[blockHash] = time.Now()
	server.recentBlocksMu.Unlock()

	server.logf("Received new block header %x (height %d) from peer %s",
		blockHash[:8], header.Height, peer.Address)

	// Check if we want this block based on our current chain
	ourChain, err := server.config.Store.GetChain()
	if err != nil {
		server.logf("Failed to get our chain: %v", err)
		return
	}

	ourHeight := uint64(0)
	if len(ourChain.Blocks) > 0 {
		ourHeight = ourChain.Blocks[len(ourChain.Blocks)-1].Header.Height
	}

	// Determine if we should request the full block
	shouldRequest := false
	reason := ""

	if header.Height == ourHeight+1 {
		// This might be the next block in sequence
		if len(ourChain.Blocks) > 0 {
			ourTip := ourChain.Blocks[len(ourChain.Blocks)-1]
			if blockchain.HashBlockHeader(&ourTip.Header) == header.PreviousHash {
				shouldRequest = true
				reason = "next block in sequence"
			}
		}
	} else if header.Height > ourHeight+1 {
		// This block is ahead of us - we might need to sync
		shouldRequest = true
		reason = "block ahead of us, might need sync"
	} else if header.Height <= ourHeight {
		// Check if this could be a fork with more work
		if len(ourChain.Blocks) > 0 {
			ourTip := ourChain.Blocks[len(ourChain.Blocks)-1]
			if blockchain.CompareWork(header.TotalWork, ourTip.Header.TotalWork) > 0 {
				shouldRequest = true
				reason = "potential fork with more work"
			}
		}
	}

	if shouldRequest {
		server.logf("Requesting full block %x: %s", blockHash[:8], reason)

		// Request the full block
		hashString := base64.StdEncoding.EncodeToString(blockHash[:])
		go func() {
			block, err := RequestBlockFromPeer(server, peer.Address, hashString)
			if err != nil {
				server.logf("Failed to request block %x from %s: %v", blockHash[:8], peer.Address, err)
				return
			}

			// Process the full block
			<-ProcessBlock(server, block, peer.Address)
		}()
	} else {
		server.logf("Ignoring block header %x (height %d, not needed)", blockHash[:8], header.Height)
	}

	// Always relay the header to other peers (except sender)
	go func() { <-RelayBlockHeader(server, header, peer.Address) }()
}

// handleNewBlock processes incoming block announcements
func handleNewBlock(server *Server, msg *Message, peer *Peer) {
	var blockPayload NewBlockPayload
	if err := msg.ParsePayload(&blockPayload); err != nil {
		server.logf("Failed to parse block payload: %v", err)
		return
	}

	// Mitigate Broadcast Storm by keeping list of blocks to ignore
	blockHash := blockchain.HashBlockHeader(&blockPayload.Block.Header)
	server.recentBlocksMu.Lock()
	if addedTime, seen := server.recentBlocks[blockHash]; seen {
		if time.Now().Sub(addedTime) <= server.recentBlocksTTL {
			server.recentBlocksMu.Unlock()
			server.logf("Ignoring new block: %x because its in recentblocks", blockHash[:8])
			return
		}
	}
	server.recentBlocks[blockHash] = time.Now()
	server.recentBlocksMu.Unlock()

	server.logf("Received new block %x from peer %s", blockHash[:8], peer.Address)

	// Process the block using the service layer (exclude the sender from relay)
	go func() { <-ProcessBlock(server, blockPayload.Block, peer.Address) }()
}

// handlePing processes ping messages and responds with pong
func handlePing(server *Server, conn net.Conn) {
	// Respond with pong
	pong := PongPayload{Timestamp: time.Now().Unix()}
	if pongMsg, err := NewMessage(MessageTypePong, pong); err == nil {
		server.sendMessage(conn, pongMsg)
	}
}

// handlePong processes pong responses
func handlePong(server *Server, peer *Peer) {
	server.logf("Received pong from peer %s", peer.Address)
}

// handleRequestPeers processes peer list requests and responds with known peers
func handleRequestPeers(server *Server, msg *Message, peer *Peer, conn net.Conn) {
	var requestPayload RequestPeersPayload
	if err := msg.ParsePayload(&requestPayload); err != nil {
		server.logf("Failed to parse peer request: %v", err)
		return
	}

	// Get list of connected peers (excluding the requester)
	server.peerManager.mu.RLock()
	var peerAddresses []string
	for addr := range server.peerManager.peers {
		if addr != peer.Address && len(peerAddresses) < requestPayload.MaxPeers {
			peerAddresses = append(peerAddresses, addr)
		}
	}
	server.peerManager.mu.RUnlock()

	// Send response
	sharePayload := SharePeersPayload{Peers: peerAddresses}
	shareMsg, err := NewMessage(MessageTypeSharePeers, sharePayload)
	if err != nil {
		server.logf("Failed to create share peers message: %v", err)
		return
	}

	// Set reply correlation if this was a request
	if msg.GetRequestID() != "" {
		shareMsg.SetReplyTo(msg.GetRequestID())
	}
	server.sendMessage(conn, shareMsg)
	server.logf("Shared %d peers with %s", len(peerAddresses), peer.Address)
}

// handleSharePeers processes received peer lists
func handleSharePeers(server *Server, msg *Message, peer *Peer) {
	var sharePayload SharePeersPayload
	if err := msg.ParsePayload(&sharePayload); err != nil {
		server.logf("Failed to parse shared peers: %v", err)
		return
	}

	server.logf("Received %d peers from %s", len(sharePayload.Peers), peer.Address)
	// The discovery component handles adding these peers
}

// handleRequestBlock processes block requests and responds with the requested block
func handleRequestBlock(server *Server, msg *Message, peer *Peer, conn net.Conn) {
	var requestBlockPayload RequestBlockPayload
	if err := msg.ParsePayload(&requestBlockPayload); err != nil {
		server.logf("Failed to parse block request: %v", err)
		return
	}

	// Get block from chain store
	chainStore := server.config.Store

	// Convert string hash to Hash32
	var blockHash blockchain.Hash32
	if err := blockHash.UnmarshalJSON([]byte(`"` + requestBlockPayload.BlockHash + `"`)); err != nil {
		server.logf("Invalid block hash format in request: %v", err)
		return
	}

	block, err := chainStore.GetBlockByHash(blockHash)
	if err != nil {
		server.logf("Failed to get requested block %s: %v", requestBlockPayload.BlockHash, err)
		return
	}

	// Send the block as response
	blockPayload := NewBlockPayload{Block: block}
	response, err := NewMessage(MessageTypeNewBlock, blockPayload)
	if err != nil {
		server.logf("Failed to create block response: %v", err)
		return
	}

	// Set reply correlation if this was a request
	if msg.GetRequestID() != "" {
		response.SetReplyTo(msg.GetRequestID())
	}

	if err := server.sendMessage(conn, response); err != nil {
		server.logf("Failed to send requested block to %s: %v", peer.Address, err)
	} else {
		server.logf("Sent requested block %s to %s", requestBlockPayload.BlockHash, peer.Address)
	}
}

// handleNewTransaction processes incoming transaction announcements
func handleNewTransaction(server *Server, msg *Message, peer *Peer) {
	var txPayload NewTxPayload
	if err := msg.ParsePayload(&txPayload); err != nil {
		server.logf("Failed to parse transaction payload: %v", err)
		return
	}

	server.logf("Received new transaction from peer %s", peer.Address)

	// TODO: Add transaction to mempool and validate
	// For now, just relay to other peers
	go func() { <-broadcastTransactionToAllExcept(server, txPayload.Transaction, peer.Address) }()
}

// handleRequestChainHead processes requests for current chain head information
func handleRequestChainHead(server *Server, msg *Message, peer *Peer, conn net.Conn) {
	var requestPayload RequestChainHeadPayload
	if err := msg.ParsePayload(&requestPayload); err != nil {
		server.logf("Failed to parse chain head request: %v", err)
		return
	}

	// Get current chain
	chain, err := server.config.Store.GetChain()
	if err != nil {
		server.logf("Failed to get chain for head request: %v", err)
		return
	}

	if len(chain.Blocks) == 0 {
		server.logf("No blocks in chain to respond with")
		return
	}

	// Get the chain head (latest block)
	headBlock := chain.Blocks[len(chain.Blocks)-1]
	headHash := blockchain.HashBlockHeader(&headBlock.Header)

	// Create chain head response
	headPayload := ChainHeadPayload{
		Height:    headBlock.Header.Height,
		HeadHash:  base64.StdEncoding.EncodeToString(headHash[:]),
		TotalWork: headBlock.Header.TotalWork,
		Header:    headBlock.Header,
	}

	response, err := NewMessage(MessageTypeChainHead, headPayload)
	if err != nil {
		server.logf("Failed to create chain head response: %v", err)
		return
	}

	// Set reply correlation
	if msg.GetRequestID() != "" {
		response.SetReplyTo(msg.GetRequestID())
	}

	if err := server.sendMessage(conn, response); err != nil {
		server.logf("Failed to send chain head to %s: %v", peer.Address, err)
	} else {
		server.logf("Sent chain head (height %d) to %s", headBlock.Header.Height, peer.Address)
	}
}

// handleChainHead processes received chain head information
func handleChainHead(server *Server, msg *Message, peer *Peer) {
	var headPayload ChainHeadPayload
	if err := msg.ParsePayload(&headPayload); err != nil {
		server.logf("Failed to parse chain head: %v", err)
		return
	}

	server.logf("Received chain head from %s: height=%d, hash=%s",
		peer.Address, headPayload.Height, headPayload.HeadHash[:16])

	// Compare with our chain and initiate IBD if needed
	ourChain, err := server.config.Store.GetChain()
	if err != nil {
		server.logf("Failed to get our chain for comparison: %v", err)
		return
	}

	ourHeight := uint64(0)
	if len(ourChain.Blocks) > 0 {
		ourHeight = ourChain.Blocks[len(ourChain.Blocks)-1].Header.Height
	}

	// If peer is significantly ahead (more than 1 block), initiate IBD
	if headPayload.Height > ourHeight+1 {
		server.logf("Peer %s is ahead by %d blocks, initiating IBD",
			peer.Address, headPayload.Height-ourHeight)

		// Start headers-first evaluation in background
		go func() {
			if err := EvaluateChainHead(server, peer.Address, &headPayload); err != nil {
				server.logf("Chain evaluation failed from %s: %v", peer.Address, err)
			} else {
				server.logf("Chain evaluation completed from %s", peer.Address)
			}
		}()
	} else if headPayload.Height == ourHeight+1 {
		// Peer is just 1 block ahead, request that specific block
		_, err := base64.StdEncoding.DecodeString(headPayload.HeadHash)
		if err != nil {
			server.logf("Failed to decode peer head hash: %v", err)
			return
		}

		server.logf("Peer %s has next block, requesting it", peer.Address)
		go func() {
			_, err := RequestBlockFromPeer(server, peer.Address, headPayload.HeadHash)
			if err != nil {
				server.logf("Failed to request next block from %s: %v", peer.Address, err)
			}
		}()
	} else {
		server.logf("Peer %s is not ahead (height %d vs our %d)", peer.Address, headPayload.Height, ourHeight)
	}
}

// handleRequestHeaders processes requests for block headers in a range
func handleRequestHeaders(server *Server, msg *Message, peer *Peer, conn net.Conn) {
	var requestPayload RequestHeadersPayload
	if err := msg.ParsePayload(&requestPayload); err != nil {
		server.logf("Failed to parse headers request: %v", err)
		return
	}

	chain, err := server.config.Store.GetChain()
	if err != nil {
		server.logf("Failed to get chain for headers request: %v", err)
		return
	}

	var headers []blockchain.BlockHeader
	maxHeaders := int(requestPayload.Count)
	if maxHeaders > 2000 { // Limit to prevent abuse
		maxHeaders = 2000
	}

	// Handle request by start height
	if requestPayload.StartHeight > 0 {
		startIdx := int(requestPayload.StartHeight)
		if startIdx < len(chain.Blocks) {
			endIdx := startIdx + maxHeaders
			if endIdx > len(chain.Blocks) {
				endIdx = len(chain.Blocks)
			}

			for i := startIdx; i < endIdx; i++ {
				headers = append(headers, chain.Blocks[i].Header)
			}
		}
	}

	// Send headers response
	headersPayload := HeadersPayload{Headers: headers}
	response, err := NewMessage(MessageTypeHeaders, headersPayload)
	if err != nil {
		server.logf("Failed to create headers response: %v", err)
		return
	}

	if msg.GetRequestID() != "" {
		response.SetReplyTo(msg.GetRequestID())
	}

	if err := server.sendMessage(conn, response); err != nil {
		server.logf("Failed to send headers to %s: %v", peer.Address, err)
	} else {
		server.logf("Sent %d headers to %s", len(headers), peer.Address)
	}
}

// handleHeaders processes received block headers
func handleHeaders(server *Server, msg *Message, peer *Peer) {
	var headersPayload HeadersPayload
	if err := msg.ParsePayload(&headersPayload); err != nil {
		server.logf("Failed to parse headers: %v", err)
		return
	}

	server.logf("Received %d headers from %s", len(headersPayload.Headers), peer.Address)

	// TODO: Validate headers and request missing blocks
	// This is where headers-first sync logic would go
}

// handleRequestBlocks processes requests for full blocks
func handleRequestBlocks(server *Server, msg *Message, peer *Peer, conn net.Conn) {
	var requestPayload RequestBlocksPayload
	if err := msg.ParsePayload(&requestPayload); err != nil {
		server.logf("Failed to parse blocks request: %v", err)
		return
	}

	chain, err := server.config.Store.GetChain()
	if err != nil {
		server.logf("Failed to get chain for blocks request: %v", err)
		return
	}

	var blocks []*blockchain.Block
	maxBlocks := int(requestPayload.Count)
	if maxBlocks > 500 { // Limit to prevent abuse
		maxBlocks = 500
	}

	// Handle request by start height and count
	if requestPayload.StartHeight > 0 && requestPayload.Count > 0 {
		startIdx := int(requestPayload.StartHeight)
		if startIdx < len(chain.Blocks) {
			endIdx := startIdx + maxBlocks
			if endIdx > len(chain.Blocks) {
				endIdx = len(chain.Blocks)
			}

			for i := startIdx; i < endIdx; i++ {
				blocks = append(blocks, chain.Blocks[i])
			}
		}
	}

	// Handle request by specific hashes
	if len(requestPayload.Hashes) > 0 {
		blocks = []*blockchain.Block{} // Clear any height-based results
		for _, hashStr := range requestPayload.Hashes {
			if len(blocks) >= maxBlocks {
				break
			}

			hashBytes, err := base64.StdEncoding.DecodeString(hashStr)
			if err != nil {
				continue
			}

			var blockHash blockchain.Hash32
			copy(blockHash[:], hashBytes)

			// Linear search through blocks (could be optimized with a map) @TODO
			for _, block := range chain.Blocks {
				if blockchain.HashBlockHeader(&block.Header) == blockHash {
					blocks = append(blocks, block)
					break
				}
			}
		}
	}

	// Send blocks response
	blocksPayload := BlocksPayload{Blocks: blocks}
	response, err := NewMessage(MessageTypeBlocks, blocksPayload)
	if err != nil {
		server.logf("Failed to create blocks response: %v", err)
		return
	}

	if msg.GetRequestID() != "" {
		response.SetReplyTo(msg.GetRequestID())
	}

	if err := server.sendMessage(conn, response); err != nil {
		server.logf("Failed to send blocks to %s: %v", peer.Address, err)
	} else {
		server.logf("Sent %d blocks to %s", len(blocks), peer.Address)
	}
}

// handleBlocks processes received full blocks
func handleBlocks(server *Server, msg *Message, peer *Peer) {
	var blocksPayload BlocksPayload
	if err := msg.ParsePayload(&blocksPayload); err != nil {
		server.logf("Failed to parse blocks: %v", err)
		return
	}

	server.logf("Received %d blocks from %s", len(blocksPayload.Blocks), peer.Address)

	// Process each block sequentially
	for _, block := range blocksPayload.Blocks {
		blockHash := blockchain.HashBlockHeader(&block.Header)
		server.logf("Processing batch block %x from %s", blockHash[:8], peer.Address)

		// Process block through the normal pipeline
		go func(b *blockchain.Block) {
			<-ProcessBlock(server, b, peer.Address)
		}(block)
	}
}
