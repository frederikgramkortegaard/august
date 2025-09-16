package networking

import (
	"august/blockchain"
	"august/consensus"
	store "august/storage"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// NetworkConfig holds network server configuration
type NetworkConfig struct {
	Port                 string
	NodeID               string
	Store                store.ChainStore
	SeedPeers            []string                                    // Seed peers for discovery
	ReqRespConfig        ReqRespConfig                               // Request-response configuration
	PeerRequestConfig    PeerRequestConfig                           // Bitcoin-style peer request configuration
	TransactionProcessor func(*blockchain.Transaction) error         // Callback to process incoming transactions
}

// Note: CandidateBlock and CandidateChain types moved to august/consensus package

// Server handles network communication and message passing
type Server struct {
	config            NetworkConfig
	listener          net.Listener
	peerManager       *PeerManager
	peerConnections   map[string]net.Conn // Active connections by peer address
	peerConnectionsMu sync.RWMutex        // Protects peerConnections map
	shutdown          chan bool           // Signal to stop server
	shutdownComplete  chan bool           // Signal that server has stopped
	reqRespClient     *ReqRespClient      // Request-response client
	recentBlocks      map[blockchain.Hash32]time.Time
	recentBlocksTTL   time.Duration
	recentBlocksMu    sync.RWMutex

	// Transaction relay tracking (same pattern as blocks)
	recentTransactions    map[blockchain.Hash32]time.Time
	recentTransactionsTTL time.Duration
	recentTransactionsMu  sync.RWMutex

	// Consensus management
	consensusManager *consensus.CandidateManager
	chainDownloader  *consensus.ChainDownloader

	// Block processing callback (set by node)
	blockProcessor func(*blockchain.Block, ...string) <-chan struct{}

	// Unified cleanup system
	cleanupTicker   *time.Ticker
	cleanupInterval time.Duration
}

// logf logs with node ID prefix
func (s *Server) logf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s\t%s", s.config.NodeID, message)
}

// IsRecentTransaction checks if we've recently seen this transaction (to prevent relay loops)
func (s *Server) IsRecentTransaction(txHash blockchain.Hash32) bool {
	s.recentTransactionsMu.RLock()
	defer s.recentTransactionsMu.RUnlock()

	if seenTime, exists := s.recentTransactions[txHash]; exists {
		// Check if it's still within TTL
		return time.Since(seenTime) < s.recentTransactionsTTL
	}
	return false
}

// MarkTransactionSeen records that we've seen this transaction
func (s *Server) MarkTransactionSeen(txHash blockchain.Hash32) {
	s.recentTransactionsMu.Lock()
	defer s.recentTransactionsMu.Unlock()

	s.recentTransactions[txHash] = time.Now()
}

// NewServer creates a new network server
func NewServer(config NetworkConfig) *Server {
	server := &Server{
		config:                config,
		peerManager:           NewPeerManager([]string{}), // Will be set by discovery
		peerConnections:       make(map[string]net.Conn),
		shutdown:              make(chan bool),
		shutdownComplete:      make(chan bool),
		recentBlocks:          make(map[blockchain.Hash32]time.Time),
		recentBlocksTTL:       5 * time.Minute,
		cleanupInterval:       30 * time.Second,
		recentTransactions:    make(map[blockchain.Hash32]time.Time),
		recentTransactionsTTL: 5 * time.Minute,
	}

	// Initialize consensus management
	server.consensusManager = consensus.NewCandidateManager(config.Store)

	// Create block request function for the chain downloader
	blockRequestFunc := func(blockHashes []string) ([]*blockchain.Block, error) {
		return RequestBlocksByHash(server, blockHashes)
	}

	// Create log function for the chain downloader
	logFunc := func(format string, args ...interface{}) {
		server.logf(format, args...)
	}

	server.chainDownloader = consensus.NewChainDownloader(server.consensusManager, blockRequestFunc, logFunc)

	// Create request-response client with this server as the sender
	server.reqRespClient = NewReqRespClient(config.ReqRespConfig, server)

	return server
}

// SendMessage implements the MessageSender interface for reqresp client
func (s *Server) SendMessage(peerAddress string, msg RequestResponse) error {
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

// StartListener begins listening for network connections (TCP only)
func (s *Server) StartListener() error {
	listener, err := net.Listen("tcp", ":"+s.config.Port)
	if err != nil {
		return err
	}

	s.listener = listener
	s.logf("Network server listening on port %s", s.config.Port)

	// Accept connections in background
	go s.acceptConnections()

	return nil
}

// StartPeriodicProcesses starts the periodic cleanup and sync processes
func (s *Server) StartPeriodicProcesses() error {
	// Start unified periodic cleanup system
	s.cleanupTicker = time.NewTicker(s.cleanupInterval)
	go s.unifiedPeriodicCleanup()

	go s.periodicChainSync()

	return nil
}

// Start begins listening for network connections and starts all periodic processes
func (s *Server) Start() error {
	if err := s.StartListener(); err != nil {
		return err
	}

	return s.StartPeriodicProcesses()
}

// unifiedPeriodicCleanup handles all periodic cleanup tasks in one place
func (s *Server) unifiedPeriodicCleanup() {
	for {
		select {
		case <-s.cleanupTicker.C:
			s.performCleanupTasks()
		case <-s.shutdown:
			if s.cleanupTicker != nil {
				s.cleanupTicker.Stop()
			}
			return
		}
	}
}

func (s *Server) performCleanupTasks() {
	now := time.Now()

	// 1. Clean up recent blocks (existing logic)
	s.cleanRecentBlocks(now)

	// 2. Clean up recent transactions (prevent memory leak)
	s.cleanRecentTransactions(now)

	// 3. Candidate chain and block cleanup is now handled by the consensus manager
	// (No cleanup needed here - consensus manager handles its own lifecycle)

	// 4. Log current chain status
	s.logChainStatus()

	s.logf("Periodic cleanup completed")
}

func (s *Server) logChainStatus() {
	chain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get chain for status: %v", err)
		return
	}

	if len(chain.Blocks) == 0 {
		s.logf("CHAIN STATUS: No blocks")
		return
	}

	latestBlock := chain.Blocks[len(chain.Blocks)-1]
	blockHash := latestBlock.Header.GetHash()
	s.logf("CHAIN STATUS: Height %d, Head %x, TxCount %d",
		latestBlock.Header.Height,
		blockHash[:8],
		len(latestBlock.Transactions))
}

func (s *Server) cleanRecentBlocks(now time.Time) {
	s.recentBlocksMu.Lock()
	defer s.recentBlocksMu.Unlock()

	cleaned := 0
	for hash, addedTime := range s.recentBlocks {
		if now.Sub(addedTime) > s.recentBlocksTTL {
			delete(s.recentBlocks, hash)
			cleaned++
		}
	}

	if cleaned > 0 {
		s.logf("Cleaned up %d old recent blocks", cleaned)
	}
}

func (s *Server) cleanRecentTransactions(now time.Time) {
	s.recentTransactionsMu.Lock()
	defer s.recentTransactionsMu.Unlock()

	cleaned := 0
	for hash, addedTime := range s.recentTransactions {
		if now.Sub(addedTime) > s.recentTransactionsTTL {
			delete(s.recentTransactions, hash)
			cleaned++
		}
	}

	if cleaned > 0 {
		s.logf("Cleaned up %d old recent transactions", cleaned)
	}
}

// periodicChainSync periodically requests chain heads from peers to detect if we need to sync
func (s *Server) periodicChainSync() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkPeerChains()
		case <-s.shutdown:
			return
		}
	}
}

// checkPeerChains requests chain heads from all connected peers to see if we need to sync
func (s *Server) checkPeerChains() {
	connectedPeers := s.peerManager.GetConnectedPeers()
	if len(connectedPeers) == 0 {
		return // No peers to sync with
	}

	s.logf("Checking chain heads from %d peers", len(connectedPeers))

	for _, peer := range connectedPeers {
		go func(peerAddr string) {
			// Request chain head from peer
			msg, err := NewMessage(MessageTypeRequestChainHead, RequestChainHeadPayload{})
			if err != nil {
				s.logf("Failed to create chain head request for %s: %v", peerAddr, err)
				return
			}

			// Use SendNotification since we'll handle the response in handleChainHead
			if err := s.reqRespClient.SendNotification(peerAddr, msg); err != nil {
				s.logf("Failed to request chain head from %s: %v", peerAddr, err)
			}
		}(peer.Address)
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

	connAddr := conn.RemoteAddr().String()
	s.logf("New peer connection from: %s", connAddr)

	// For incoming connections, we don't know the peer's listening address yet
	// We'll get it from their handshake. For now, create a temporary peer entry
	peer := &Peer{
		Address:    connAddr, // Will be updated to listen address after handshake
		ConnAddr:   connAddr, // Actual connection endpoint
		Status:     PeerConnecting,
		IsOutgoing: false, // This is an incoming connection
	}

	// Store the connection using the connection address
	s.peerConnectionsMu.Lock()
	s.peerConnections[connAddr] = conn
	s.peerConnectionsMu.Unlock()

	defer func() {
		// Clean up the connection when done
		s.peerConnectionsMu.Lock()
		delete(s.peerConnections, connAddr)
		// Also try to delete by peer's actual address if it changed
		if peer.Address != connAddr {
			delete(s.peerConnections, peer.Address)
		}
		s.peerConnectionsMu.Unlock()

		// Mark peer as disconnected if it's in the manager
		s.peerManager.mu.Lock()
		if p, exists := s.peerManager.peers[peer.Address]; exists {
			p.Status = PeerDisconnected
		}
		s.peerManager.mu.Unlock()
	}()

	// Send handshake
	s.logf("Sending handshake to %s", connAddr)
	s.sendHandshake(connAddr)

	// Handle incoming messages
	s.logf("Starting message handler for %s", connAddr)
	s.handleMessages(conn, peer)
}

// sendHandshake sends initial handshake to a peer
func (s *Server) sendHandshake(peerAddr string) {
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

	s.logf("Sending handshake message to %s (port: %s)", peerAddr, handshake.ListenPort)
	if err := s.reqRespClient.SendNotification(peerAddr, msg); err != nil {
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
		ProcessMessage(s, &msg, peer, conn)
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

// GetConsensusManager returns the consensus manager
func (s *Server) GetConsensusManager() *consensus.CandidateManager {
	return s.consensusManager
}

// SetBlockProcessor sets the callback function for processing blocks
func (s *Server) SetBlockProcessor(processor func(*blockchain.Block, ...string) <-chan struct{}) {
	s.blockProcessor = processor
}

// Note: Candidate chains and blocks are now managed by the consensus package
// Use server.consensusManager to access candidate functionality

// GetChainStore returns the chain store for testing purposes
func (s *Server) GetChainStore() store.ChainStore {
	return s.config.Store
}

// Stop gracefully shuts down the network server
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

// StartDiscovery begins peer discovery process
func (s *Server) StartDiscovery() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		s.logf("Starting peer discovery with %d seed peers", len(s.config.SeedPeers))

		// Connect to seed peers
		go s.connectToSeeds()

		// Periodic peer discovery
		go s.periodicDiscovery()

		s.logf("Peer discovery started successfully")
		ready <- true
	}()

	return ready
}

// connectToSeeds attempts to connect to all seed peers and waits for completion
func (s *Server) connectToSeeds() <-chan bool {
	done := make(chan bool, 1)

	go func() {
		if len(s.config.SeedPeers) == 0 {
			done <- true
			return
		}

		// Get current peers to avoid duplicate connections
		pm := s.GetPeerManager()
		pm.mu.RLock()
		currentPeers := make(map[string]bool)
		for addr := range pm.peers {
			currentPeers[addr] = true
		}
		pm.mu.RUnlock()

		var connectionTasks []<-chan bool
		for _, seedAddr := range s.config.SeedPeers {
			if !currentPeers[seedAddr] {
				connectionTasks = append(connectionTasks, s.ConnectToPeer(seedAddr))
			} else {
				s.logf("Skipping seed %s - already connected", seedAddr)
			}
		}

		// Wait for all connection attempts to complete
		successCount := 0
		for _, task := range connectionTasks {
			if <-task {
				successCount++
			}
		}

		s.logf("Seed connection attempts completed: %d/%d successful", successCount, len(s.config.SeedPeers))
		done <- true
	}()

	return done
}

// connectToDiscoveredPeers attempts to connect to peers we've learned about
func (s *Server) connectToDiscoveredPeers() <-chan bool {
	done := make(chan bool, 1)

	go func() {
		pm := s.GetPeerManager()
		discoveredPeers := pm.GetDiscoveredPeers()

		// Get current peers to avoid duplicate connections
		pm.mu.RLock()
		currentPeers := make(map[string]bool)
		for addr := range pm.peers {
			currentPeers[addr] = true
		}
		pm.mu.RUnlock()

		// Try connecting to discovered peers we're not already connected to
		var connectionTasks []<-chan bool
		connected := 0
		maxConnections := s.config.PeerRequestConfig.MaxPeersForRequest
		for _, addr := range discoveredPeers {
			if !currentPeers[addr] && connected < maxConnections {
				connectionTasks = append(connectionTasks, s.ConnectToPeer(addr))
				connected++
			}
		}

		if connected > 0 {
			s.logf("Attempting to connect to %d discovered peers", connected)
			// Wait for all connection attempts to complete
			successCount := 0
			for _, task := range connectionTasks {
				if <-task {
					successCount++
				}
			}
			s.logf("Discovery connection attempts completed: %d/%d successful", successCount, connected)
		}

		done <- true
	}()

	return done
}

func (s *Server) requestPeerSharing() {
	pm := s.GetPeerManager()
	if pm == nil {
		return
	}

	// Get connected peers
	connectedPeers := pm.GetConnectedPeers()
	if len(connectedPeers) == 0 {
		return
	}

	// Request peers from all connected peers and collect responses
	allDiscoveredPeers := make([]string, 0)
	successfulRequests := 0

	// Use smart peer discovery instead of iterating through individual peers
	peers, err := RequestPeers(s, 50)
	if err != nil {
		s.logf("Failed to discover peers: %v", err)
	} else {
		allDiscoveredPeers = append(allDiscoveredPeers, peers...)
		successfulRequests = 1 // Simplified since we're doing one aggregated request
	}

	if successfulRequests > 0 {
		s.logf("Successfully requested peers from %d peers", successfulRequests)

		// Add all discovered peers to our peer manager
		if len(allDiscoveredPeers) > 0 {
			newPeerCount := pm.AddDiscoveredPeers(allDiscoveredPeers)
			if newPeerCount > 0 {
				s.logf("Discovered %d new peers through peer sharing", newPeerCount)
			}
		}
	}
}

// requestPeerSharingAndConnect requests peers and then tries to connect to them
func (s *Server) requestPeerSharingAndConnect() <-chan bool {
	done := make(chan bool, 1)

	go func() {
		s.logf("Starting peer sharing and connect sequence")
		// Request peers (synchronous - responses are handled immediately)
		s.requestPeerSharing()

		// Now try to connect to any newly discovered peers
		s.logf("Now attempting to connect to discovered peers")
		<-s.connectToDiscoveredPeers()

		done <- true
	}()

	return done
}

// connectToPeer attempts to connect to a specific peer and waits for handshake completion
func (s *Server) ConnectToPeer(address string) <-chan bool {
	result := make(chan bool, 1)

	go func() {
		defer func() { result <- false }() // Default to failure

		// Check if we're already connected to this peer
		pm := s.GetPeerManager()
		pm.mu.RLock()
		existingPeer, exists := pm.peers[address]
		if exists && existingPeer.Status == PeerConnected {
			pm.mu.RUnlock()
			s.logf("Already connected to peer %s, skipping connection attempt", address)
			result <- true // Already connected counts as success
			return
		}
		pm.mu.RUnlock()

		s.logf("Attempting to connect to peer: %s", address)

		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			s.logf("Failed to connect to peer %s: %v", address, err)
			return
		}

		// Create peer entry for outgoing connection
		peer := &Peer{
			Address:    address,                   // Listen address (where we're connecting to)
			ConnAddr:   conn.LocalAddr().String(), // Our side of the connection
			Status:     PeerConnecting,
			IsOutgoing: true, // We initiated this connection
		}

		// Add to peer manager
		actualPeer := s.GetPeerManager().AddPeer(address)
		if actualPeer != nil {
			// Update the peer object with our info
			actualPeer.ConnAddr = conn.LocalAddr().String()
			actualPeer.IsOutgoing = true
			peer = actualPeer
			s.logf("TCP connection established to peer: %s", address)

			// For outgoing connections, we handle it differently
			// Store the connection
			s.peerConnectionsMu.Lock()
			s.peerConnections[address] = conn
			s.peerConnectionsMu.Unlock()

			// Send handshake immediately
			s.sendHandshake(address)

			// Start message handler (but NOT HandlePeerConnection as that's for incoming)
			go func() {
				defer conn.Close()
				defer func() {
					s.peerConnectionsMu.Lock()
					delete(s.peerConnections, address)
					s.peerConnectionsMu.Unlock()
					peer.Status = PeerDisconnected
				}()

				peer.Status = PeerConnected
				s.handleMessages(conn, peer)
			}()

			// Wait for handshake to complete using channels
			handshakeComplete := make(chan bool, 1)

			go func() {
				// Monitor for handshake completion
				timeout := time.NewTimer(10 * time.Second) // Increased timeout for race conditions
				defer timeout.Stop()

				ticker := time.NewTicker(50 * time.Millisecond)
				defer ticker.Stop()

				for {
					select {
					case <-timeout.C:
						s.logf("Handshake timeout for peer %s", address)
						handshakeComplete <- false
						return
					case <-ticker.C:
						// Check if ANY connection to this peer exists (not just the one we initiated)
						pm := s.GetPeerManager()
						pm.mu.RLock()
						if peerObj, exists := pm.peers[address]; exists && peerObj.Status == PeerConnected {
							pm.mu.RUnlock()
							s.logf("Peer connection established: %s (may be incoming due to race)", address)
							handshakeComplete <- true
							return
						}
						pm.mu.RUnlock()
					}
				}
			}()

			// Wait for handshake result
			if <-handshakeComplete {
				result <- true
				return
			}
		} else {
			// If we can't add the peer, close the connection
			conn.Close()
		}
	}()

	return result
}

// periodicDiscovery runs periodic peer discovery
func (s *Server) periodicDiscovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Just trigger a manual discovery round
		<-s.RunDiscoveryRound()
	}
}

// RunDiscoveryRound manually triggers one round of peer discovery
// Returns a channel that signals when the discovery round is complete
func (s *Server) RunDiscoveryRound() <-chan bool {
	done := make(chan bool, 1)

	go func() {
		defer func() { done <- true }()

		pm := s.GetPeerManager()
		connected := pm.GetConnectedPeers()
		var connectedAddrs []string
		for _, peer := range connected {
			connectedAddrs = append(connectedAddrs, peer.Address)
		}
		s.logf("Manual discovery check: %d connected peers: %v", len(connected), connectedAddrs)

		// Clean up dead peers
		removed := pm.CleanupDeadPeers()
		if removed > 0 {
			s.logf("Cleaned up %d dead peers", removed)
		}

		// Different strategies based on connection count
		if len(connected) == 0 {
			// No connections, try seed peers
			s.logf("No connected peers, attempting to connect to seed peers")
			<-s.connectToSeeds() // Wait for seed connections to complete
		} else if len(connected) < 15 {
			// Few connections, try to get more
			<-s.connectToDiscoveredPeers()
			<-s.requestPeerSharingAndConnect()
		} else if len(connected) < 20 {
			<-s.connectToDiscoveredPeers()
		}

		s.logf("Discovery round completed")
	}()

	return done
}

// shouldKeepExistingConnection determines which connection to keep when duplicates are detected
// Uses a deterministic rule based on node IDs to ensure both nodes make the same decision
func (s *Server) shouldKeepExistingConnection(existingPeer *Peer, newPeer *Peer, remoteNodeID string) bool {
	// Use lexicographic comparison of node IDs to make consistent decisions across nodes
	// The node with the smaller ID keeps its outgoing connection
	localNodeID := s.config.NodeID

	if localNodeID < remoteNodeID {
		// We have the smaller ID, so we keep our outgoing connections
		return existingPeer.IsOutgoing
	} else {
		// Remote has the smaller ID, so we keep incoming connections (their outgoing)
		return !existingPeer.IsOutgoing
	}
}
