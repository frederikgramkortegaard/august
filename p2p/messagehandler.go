package p2p

import (
	"net"
	"strings"
	"time"

	"gocuria/blockchain"
)

// ProcessMessage handles different types of P2P messages
func ProcessMessage(server *Server, msg *Message, peer *Peer, conn net.Conn) {
	// First check if this is a response to a pending request
	if handled := server.reqRespClient.HandleResponse(msg); handled {
		server.logf("Delivered response for request %s", msg.ReplyTo)
		return
	}

	switch msg.Type {
	case MessageTypeHandshake:
		handleHandshake(server, msg, peer, conn)
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