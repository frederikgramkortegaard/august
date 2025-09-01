package p2p

import (
	"encoding/json"
	"fmt"
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"gocuria/p2p/reqresp"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// BlockProcessor defines the interface for processing blocks (avoids circular imports)
type BlockProcessor interface {
	ProcessBlock(block *blockchain.Block) error
}

// Config holds P2P server configuration
type Config struct {
	Port           string
	NodeID         string
	Store          store.ChainStore
	BlockProcessor BlockProcessor
	ReqRespConfig  reqresp.Config // Request-response configuration
}

// DefaultReqRespConfig returns default request-response configuration
func DefaultReqRespConfig() reqresp.Config {
	return reqresp.DefaultConfig()
}

// Server handles P2P networking and message passing
type Server struct {
	config            Config
	listener          net.Listener
	peerManager       *PeerManager
	peerConnections   map[string]net.Conn // Active connections by peer address
	peerConnectionsMu sync.RWMutex        // Protects peerConnections map
	shutdown          chan bool           // Signal to stop server
	shutdownComplete  chan bool           // Signal that server has stopped
	reqRespClient     *reqresp.Client     // Request-response client
	recentBlocks map[blockchain.Hash32]time.Time
	recentBlocksTTL time.Duration
		recentBlocksMu		sync.RWMutex
}

// logf logs with node ID prefix
func (s *Server) logf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s\t%s", s.config.NodeID, message)
}

// NewServer creates a new P2P server
func NewServer(config Config) *Server {
	server := &Server{
		config:           config,
		peerManager:      NewPeerManager([]string{}), // Will be set by discovery
		peerConnections:  make(map[string]net.Conn),
		shutdown:         make(chan bool),
		shutdownComplete: make(chan bool),
		recentBlocks: 		make(map[blockchain.Hash32]time.Time),
		recentBlocksTTL: 5*time.Minute,
	}
	
	// Create request-response client with this server as the sender
	server.reqRespClient = reqresp.NewClient(config.ReqRespConfig, server)
	
	return server
}

// SendMessage implements the MessageSender interface for reqresp client
func (s *Server) SendMessage(peerAddress string, msg reqresp.RequestResponse) error {
	// Get the connection for this peer
	s.peerConnectionsMu.RLock()
	conn, exists := s.peerConnections[peerAddress]
	s.peerConnectionsMu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no connection to peer %s", peerAddress)
	}
	
	// Cast to *Message and send
	message, ok := msg.(*Message)
	if !ok {
		return fmt.Errorf("invalid message type")
	}
	
	return s.sendMessage(conn, message)
}

// Start begins listening for P2P connections
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", ":"+s.config.Port)
	if err != nil {
		return err
	}

	s.listener = listener
	s.logf("P2P server listening on port %s", s.config.Port)

	// Accept connections in background
	go s.acceptConnections()

	go s.periodicRecentBlocksCleanup()

	return nil
}

func (s *Server) cleanupRecentBlocks() {
	now := time.Now()
	s.recentBlocksMu.Lock()
	defer s.recentBlocksMu.Unlock()
	for hash, addedTime := range s.recentBlocks {
		if now.Sub(addedTime) > s.recentBlocksTTL {
			delete(s.recentBlocks, hash)
		}
	}
}
func (s *Server) periodicRecentBlocksCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupRecentBlocks()
		case <-s.shutdown:
			return
		}
	}
}

// acceptConnections handles incoming peer connections
func (s *Server) acceptConnections() {
	defer func() {
		s.shutdownComplete <- true
	}()

	for {
		// Set a short accept timeout to allow checking shutdown signal
		conn, err := s.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.shutdown:
				// Shutdown requested, exit gracefully without logging error
				return
			default:
				// Only log if it's not a shutdown-related error
				if !isNetworkClosedError(err) {
					s.logf("Failed to accept connection: %v", err)
				}
				return
			}
		}

		// Check for shutdown before handling connection
		select {
		case <-s.shutdown:
			conn.Close()
			return
		default:
			// Handle peer connection in goroutine
			go s.HandlePeerConnection(conn)
		}
	}
}

// isNetworkClosedError checks if error is due to closed network connection
func isNetworkClosedError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}

// HandlePeerConnection manages communication with a connected peer (public for discovery)
func (s *Server) HandlePeerConnection(conn net.Conn) {
	defer conn.Close()

	peerAddr := conn.RemoteAddr().String()
	s.logf("New peer connection from: %s", peerAddr)

	peer := s.peerManager.AddPeer(peerAddr)
	if peer == nil {
		s.logf("Failed to add peer %s (peer limit or already exists)", peerAddr)
		return
	}

	peer.Status = PeerConnected
	s.logf("Peer %s connected", peerAddr)

	// Store the connection for later message sending
	s.peerConnectionsMu.Lock()
	s.peerConnections[peerAddr] = conn
	s.peerConnectionsMu.Unlock()

	defer func() {
		s.peerConnectionsMu.Lock()
		delete(s.peerConnections, peerAddr)
		s.peerConnectionsMu.Unlock()
		peer.Status = PeerDisconnected
	}()

	// Send handshake
	s.logf("Sending handshake to %s", peerAddr)
	s.sendHandshake(conn)

	// Handle incoming messages
	s.logf("Starting message handler for %s", peerAddr)
	s.handleMessages(conn, peer)
}

// sendHandshake sends initial handshake to a peer
func (s *Server) sendHandshake(conn net.Conn) {
	height, err := s.config.Store.GetChainHeight()
	if err != nil {
		s.logf("Failed to get chain height: %v", err)
		height = 0
	}

	handshake := HandshakePayload{
		NodeID:      s.config.NodeID,
		ChainHeight: int(height),
		Version:     "1.0",
		ListenPort:  s.config.Port,
	}

	msg, err := NewMessage(MessageTypeHandshake, handshake)
	if err != nil {
		s.logf("Failed to create handshake message: %v", err)
		return
	}

	s.logf("Sending handshake message to %s (port: %s)", conn.RemoteAddr().String(), handshake.ListenPort)
	if err := s.sendMessage(conn, msg); err != nil {
		s.logf("Failed to send handshake: %v", err)
	}
}

// sendMessage sends a message over the connection
func (s *Server) sendMessage(conn net.Conn, msg *Message) error {
	encoder := json.NewEncoder(conn)
	err := encoder.Encode(msg)
	if err != nil {
		s.logf("Failed to encode/send message %s: %v", msg.Type, err)
	}
	return err
}

// handleMessages processes incoming messages from a peer
func (s *Server) handleMessages(conn net.Conn, peer *Peer) {
	decoder := json.NewDecoder(conn)

	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				s.logf("Peer %s disconnected", peer.Address)
				peer.Status = PeerDisconnected
			} else if strings.Contains(err.Error(), "connection reset") {
				s.logf("Peer %s connection reset", peer.Address)
				peer.Status = PeerDisconnected
			} else {
				s.logf("Error decoding message from peer %s: %v", peer.Address, err)
				peer.Status = PeerFailed
			}
			return
		}

		s.logf("Received message type %s from %s", msg.Type, peer.Address)
		s.processMessage(&msg, peer, conn)
	}
}

// processMessage handles different types of P2P messages
func (s *Server) processMessage(msg *Message, peer *Peer, conn net.Conn) {
	// First check if this is a response to a pending request
	if handled := s.reqRespClient.HandleResponse(msg); handled {
		s.logf("Delivered response for request %s", msg.ReplyTo)
		return
	}

	switch msg.Type {
	case MessageTypeHandshake:
		var handshake HandshakePayload
		if err := msg.ParsePayload(&handshake); err != nil {
			s.logf("Failed to parse handshake: %v", err)
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
		s.logf("Handshake debug: current=%s, proper=%s, listenPort=%s", peer.Address, properPeerAddr, handshake.ListenPort)
		
		// Check for duplicate connections (bidirectional connection detection)
		s.peerManager.mu.Lock()
		existingPeer, exists := s.peerManager.peers[properPeerAddr]
		if exists && existingPeer != peer {
			// We have a duplicate connection!
			// Determine which connection to keep based on address comparison
			localListenAddr := net.JoinHostPort(host, s.config.Port) // Use same IP, our port
			
			s.logf("Duplicate connection detected: local=%s, remote=%s", localListenAddr, properPeerAddr)
			
			if properPeerAddr < localListenAddr {
				// Keep the incoming connection, close the existing outgoing one
				s.logf("Keeping incoming connection from %s (lower address)", properPeerAddr)
				
				// Close existing connection
				s.peerConnectionsMu.Lock()
				if oldConn, hasConn := s.peerConnections[properPeerAddr]; hasConn {
					oldConn.Close()
				}
				s.peerConnectionsMu.Unlock()
				
				// Replace the existing peer with this connection
				existingPeer.Status = PeerDisconnected
				delete(s.peerManager.peers, peer.Address) // Remove ephemeral address entry
				peer.Address = properPeerAddr
				s.peerManager.peers[properPeerAddr] = peer
			} else {
				// Keep the existing outgoing connection, close this incoming one
				s.logf("Rejecting incoming connection from %s (keeping outgoing)", properPeerAddr)
				s.peerManager.mu.Unlock()
				conn.Close()
				return
			}
		} else if peer.Address != properPeerAddr {
			// No duplicate, just update address
			s.logf("Updating peer address from %s to %s", peer.Address, properPeerAddr)
			
			// Remove the old address
			delete(s.peerManager.peers, peer.Address)
			// Add with new address
			peer.Address = properPeerAddr
			s.peerManager.peers[properPeerAddr] = peer
		}
		s.peerManager.mu.Unlock()
		
		// Update connection map
		s.peerConnectionsMu.Lock()
		delete(s.peerConnections, remoteAddr)
		s.peerConnections[properPeerAddr] = conn
		s.peerConnectionsMu.Unlock()
		
		s.logf("Received handshake from %s (height: %d)", handshake.NodeID, handshake.ChainHeight)

	case MessageTypeNewBlock:
		var blockPayload NewBlockPayload
		if err := msg.ParsePayload(&blockPayload); err != nil {
			s.logf("Failed to parse new block: %v", err)
			return
		}
		blockHash := blockchain.HashBlockHeader(&blockPayload.Block.Header)

		// Mitigate Broadcast Storm by keeping list of blocks to ignore
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

		// Process block using the FullNode's ProcessBlock method (handles orphans)
		if s.config.BlockProcessor != nil {
			if err := s.config.BlockProcessor.ProcessBlock(blockPayload.Block); err != nil {
				s.logf("Failed to process block %x from peer %s: %v", blockHash[:8], peer.Address, err)
			} else {
				s.logf("Successfully processed block %x from peer %s", blockHash[:8], peer.Address)

				// Relay the block to all other peers (except the sender to avoid loops)
				s.relayBlockToOthers(blockPayload.Block, peer.Address)
			}
		} else {
			s.logf("No block processor configured, cannot process block %x", blockHash[:8])
		}

	case MessageTypePing:
		// Respond with pong
		pong := PongPayload{Timestamp: time.Now().Unix()}
		if pongMsg, err := NewMessage(MessageTypePong, pong); err == nil {
			s.sendMessage(conn, pongMsg)
		}

	case MessageTypePong:
		s.logf("Received pong from peer %s", peer.Address)

	case MessageTypeRequestPeers:
		// Handle peer list request
		var requestPayload RequestPeersPayload
		if err := msg.ParsePayload(&requestPayload); err != nil {
			s.logf("Failed to parse request peers payload: %v", err)
			return
		}
		
		// Get connected peers to share
		connectedPeers := s.peerManager.GetConnectedPeers()
		
		// Limit the number of peers to share
		maxPeers := requestPayload.MaxPeers
		if maxPeers <= 0 || maxPeers > 256 {
			maxPeers = 256
		}
		
		// Build list of peer addresses
		peerAddresses := make([]string, 0, maxPeers)
		for _, p := range connectedPeers {
			if len(peerAddresses) >= maxPeers {
				break
			}
			// Don't share the requesting peer's own address back to them
			if p.Address != peer.Address {
				peerAddresses = append(peerAddresses, p.Address)
			}
		}
		
		// Send response
		sharePayload := SharePeersPayload{Peers: peerAddresses}
		if shareMsg, err := NewMessage(MessageTypeSharePeers, sharePayload); err == nil {
			// Set ReplyTo field if this was a request with an ID
			if msg.GetRequestID() != "" {
				shareMsg.SetReplyTo(msg.GetRequestID())
			}
			s.sendMessage(conn, shareMsg)
			s.logf("Shared %d peers with %s", len(peerAddresses), peer.Address)
		}

	case MessageTypeSharePeers:
		// Handle received peer list
		var sharePayload SharePeersPayload
		if err := msg.ParsePayload(&sharePayload); err != nil {
			s.logf("Failed to parse share peers payload: %v", err)
			return
		}
		
		// Store these peers for the next discovery cycle to connect to
		if len(sharePayload.Peers) > 0 {
			newPeerCount := s.peerManager.AddDiscoveredPeers(sharePayload.Peers)
			if newPeerCount > 0 {
				s.logf("Received %d new peers from %s", newPeerCount, peer.Address)
			}
	}

	case MessageTypeRequestBlock:
		// if the request block exists, send it back to the requester as a regular NewBlock message
		var requestBlockPayload RequestBlockPayload 
		if err := msg.ParsePayload(&requestBlockPayload); err != nil {
			s.logf("Failed to parse request block payload: %v", err)
			return
		}
		
		// Convert string hash to Hash32
		var blockHash blockchain.Hash32
		if err := blockHash.UnmarshalJSON([]byte(`"` + requestBlockPayload.BlockHash + `"`)); err != nil {
			s.logf("Invalid block hash format in request: %v", err)
			return
		}
		
		// Look up the block in our chain store
		block, err := s.config.Store.GetBlockByHash(blockHash)
		if err != nil {
			s.logf("Block %s not found in our chain", requestBlockPayload.BlockHash)
			return
		}
		
		// Send the block back as a NewBlock message
		blockPayload := NewBlockPayload{Block: block}
		response, err := NewMessage(MessageTypeNewBlock, blockPayload)
		if err != nil {
			s.logf("Failed to create block response message: %v", err)
			return
		}
		
		// Set ReplyTo field if this was a request with an ID
		if msg.GetRequestID() != "" {
			response.SetReplyTo(msg.GetRequestID())
		}
		
		if err := s.sendMessage(conn, response); err != nil {
			s.logf("Failed to send requested block to %s: %v", peer.Address, err)
		} else {
			s.logf("Sent requested block %s to %s", requestBlockPayload.BlockHash, peer.Address)
		}

	case MessageTypeNewTx:
		// Handle incoming transaction
		var txPayload NewTxPayload
		if err := msg.ParsePayload(&txPayload); err != nil {
			s.logf("Failed to parse transaction payload: %v", err)
			return
		}
		
		s.logf("Received transaction from peer %s", peer.Address)
		
		// TODO: Validate transaction and add to mempool
		// For now, just relay to other peers to avoid loops
		s.broadcastTransactionToAllExcept(txPayload.Transaction, peer.Address)

	default:
		s.logf("Unknown message type: %s", msg.Type)
	}
}

// GetListener returns the server listener (for testing)
func (s *Server) GetListener() net.Listener {
	return s.listener
}

// GetPeerManager returns the peer manager (for testing)
func (s *Server) GetPeerManager() *PeerManager {
	return s.peerManager
}

// GetChainStore returns the chain store for testing purposes
func (s *Server) GetChainStore() store.ChainStore {
	return s.config.Store
}

// GetBlockProcessor returns the block processor for testing purposes
func (s *Server) GetBlockProcessor() BlockProcessor {
	return s.config.BlockProcessor
}

// Stop gracefully shuts down the P2P server
func (s *Server) Stop() error {
	if s.listener != nil {
		// Close the listener first
		s.listener.Close()
	}

	// Signal shutdown
	close(s.shutdown)

	// Wait for acceptConnections to finish
	<-s.shutdownComplete

	// Close all peer connections
	s.peerConnectionsMu.Lock()
	for addr, conn := range s.peerConnections {
		conn.Close()
		delete(s.peerConnections, addr)
	}
	s.peerConnectionsMu.Unlock()

	return nil
}

// relayBlockToOthers broadcasts a block to all connected peers except the sender
func (s *Server) relayBlockToOthers(block *blockchain.Block, excludePeerAddr string) {
	blockHash := blockchain.HashBlockHeader(&block.Header)
	s.logf("Relaying block %x to other peers (excluding %s)", blockHash[:8], excludePeerAddr)

	// Create the block payload
	blockPayload := NewBlockPayload{Block: block}
	msg, err := NewMessage(MessageTypeNewBlock, blockPayload)
	if err != nil {
		s.logf("Failed to create relay message for block %x: %v", blockHash[:8], err)
		return
	}

	// Send to all connected peers except the sender
	relayCount := 0
	for _, peer := range s.peerManager.GetConnectedPeers() {
		if peer.Address != excludePeerAddr && peer.Status == PeerConnected {
			// Get the stored connection for this peer (with read lock)
			s.peerConnectionsMu.RLock()
			conn, exists := s.peerConnections[peer.Address]
			s.peerConnectionsMu.RUnlock()

			if exists {
				if err := s.sendMessage(conn, msg); err != nil {
					s.logf("Failed to relay block %x to peer %s: %v", blockHash[:8], peer.Address, err)
				} else {
					s.logf("Relayed block %x to peer %s", blockHash[:8], peer.Address)
					relayCount++
				}
			} else {
				s.logf("No active connection for peer %s, skipping relay", peer.Address)
			}
		}
	}

	s.logf("Successfully relayed block %x to %d peers", blockHash[:8], relayCount)
}

// RelayBlock is a public method to relay blocks to all connected peers
func (s *Server) RelayBlock(block *blockchain.Block) {
	// Use empty string as excludePeerAddr since this block was added locally
	s.relayBlockToOthers(block, "")
}

// SendPeerRequest sends a request for peers to a specific peer
func (s *Server) SendPeerRequest(peerAddress string, maxPeers int) error {
	// Get the connection for this peer
	s.peerConnectionsMu.RLock()
	conn, exists := s.peerConnections[peerAddress]
	s.peerConnectionsMu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no connection to peer %s", peerAddress)
	}
	
	// Send request
	requestPayload := RequestPeersPayload{MaxPeers: maxPeers}
	msg, err := NewMessage(MessageTypeRequestPeers, requestPayload)
	if err != nil {
		return fmt.Errorf("failed to create request message: %w", err)
	}
	
	if err := s.sendMessage(conn, msg); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	
	s.logf("Sent peer request to %s (max: %d)", peerAddress, maxPeers)
	return nil
}

// RequestBlockFromPeer requests a block and waits for the response
func (s *Server) RequestBlockFromPeer(peerAddress string, blockHash string) (*blockchain.Block, error) {
	requestPayload := RequestBlockPayload{BlockHash: blockHash}
	
	msg, err := NewMessage(MessageTypeRequestBlock, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request message: %w", err)
	}
	
	response, err := s.reqRespClient.SendRequest(peerAddress, msg)
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
	
	s.logf("Received block %s from %s", blockHash, peerAddress)
	return blockPayload.Block, nil
}

// RequestPeersFromPeer requests peers and waits for the response
func (s *Server) RequestPeersFromPeer(peerAddress string, maxPeers int) ([]string, error) {
	requestPayload := RequestPeersPayload{MaxPeers: maxPeers}
	
	msg, err := NewMessage(MessageTypeRequestPeers, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request message: %w", err)
	}
	
	response, err := s.reqRespClient.SendRequest(peerAddress, msg)
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
	
	s.logf("Received %d peers from %s", len(sharePayload.Peers), peerAddress)
	return sharePayload.Peers, nil
}

// BroadcastTransaction broadcasts a transaction to all connected peers
func (s *Server) BroadcastTransaction(tx *blockchain.Transaction) error {
	s.broadcastTransactionToAllExcept(tx, "")
	return nil
}

// broadcastTransactionToAllExcept broadcasts a transaction to all connected peers except the specified one
func (s *Server) broadcastTransactionToAllExcept(tx *blockchain.Transaction, excludePeerAddr string) {
	// Create the transaction payload
	txPayload := NewTxPayload{Transaction: tx}
	msg, err := NewMessage(MessageTypeNewTx, txPayload)
	if err != nil {
		s.logf("Failed to create transaction message: %v", err)
		return
	}
	
	if excludePeerAddr == "" {
		s.logf("Broadcasting transaction to all peers")
	} else {
		s.logf("Relaying transaction to other peers (excluding %s)", excludePeerAddr)
	}
	
	// Send to all connected peers except the excluded one
	sentCount := 0
	for _, peer := range s.peerManager.GetConnectedPeers() {
		if peer.Address != excludePeerAddr && peer.Status == PeerConnected {
			s.peerConnectionsMu.RLock()
			conn, exists := s.peerConnections[peer.Address]
			s.peerConnectionsMu.RUnlock()
			
			if exists {
				if err := s.sendMessage(conn, msg); err != nil {
					s.logf("Failed to send transaction to peer %s: %v", peer.Address, err)
				} else {
					s.logf("Sent transaction to peer %s", peer.Address)
					sentCount++
				}
			}
		}
	}
	
	s.logf("Successfully sent transaction to %d peers", sentCount)
}
