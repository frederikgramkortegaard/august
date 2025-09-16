package networking

import (
	"august/blockchain"
	"august/config"
	"august/consensus"
	"august/storage"
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
	Store                storage.ChainStore
	SeedPeers            []string                                    // Seed peers for discovery
	PeerRequestConfig    PeerRequestConfig                           // Bitcoin-style peer request configuration
	TransactionProcessor func(*blockchain.Transaction) error         // Callback to process incoming transactions
}

// Note: CandidateBlock and CandidateChain types moved to august/consensus package

// Server handles network communication and message passing
type Server struct {
	config            NetworkConfig
	listener          net.Listener
	// Peer management (integrated directly)
	peers             map[string]*Peer
	seedPeers         []string
	discoveredPeers   []string
	peersMu           sync.RWMutex        // Protects peers map and discoveredPeers
	peerConnections   map[string]net.Conn // Active connections by peer address
	peerConnectionsMu sync.RWMutex        // Protects peerConnections map
	shutdown          chan bool           // Signal to stop server
	shutdownComplete  chan bool           // Signal that server has stopped
	// Request-response handling (integrated directly)
	pendingRequests   map[string]chan *Message
	pendingMutex      sync.RWMutex
	recentBlocks      map[blockchain.Hash32]time.Time
	recentBlocksMu    sync.RWMutex

	// Transaction relay tracking (same pattern as blocks)
	recentTransactions    map[blockchain.Hash32]time.Time
	recentTransactionsMu  sync.RWMutex

	// Consensus management
	consensusManager *consensus.CandidateManager
	chainDownloader  *consensus.ChainDownloader

	// Block processing callback (set by node)
	blockProcessor func(*blockchain.Block, ...string) <-chan struct{}

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
		return time.Since(seenTime) < config.RecentTransactionsTTL
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
		// Initialize peer management
		peers:                 make(map[string]*Peer),
		seedPeers:             config.SeedPeers,
		discoveredPeers:       make([]string, 0),
		peerConnections:       make(map[string]net.Conn),
		shutdown:              make(chan bool),
		shutdownComplete:      make(chan bool),
		recentBlocks:          make(map[blockchain.Hash32]time.Time),
		recentTransactions:    make(map[blockchain.Hash32]time.Time),
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

	// Initialize request-response handling
	server.pendingRequests = make(map[string]chan *Message)

	return server
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


// Start begins listening for network connections and starts all periodic processes
func (s *Server) Start() error {
	if err := s.StartListener(); err != nil {
		return err
	}

	return s.StartPeriodicProcesses()
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

		// Mark peer as disconnected
		s.peersMu.Lock()
		if p, exists := s.peers[peer.Address]; exists {
			p.Status = PeerDisconnected
		}
		s.peersMu.Unlock()
	}()

	// Send handshake
	s.logf("Sending handshake to %s", connAddr)
	s.sendHandshake(connAddr)

	// Handle incoming messages
	s.logf("Starting message handler for %s", connAddr)
	s.handleMessages(conn, peer)
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
		s.ProcessMessage(&msg, peer, conn)
	}
}

// GetListener returns the server listener (for testing)
func (s *Server) GetListener() net.Listener {
	return s.listener
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
func (s *Server) GetChainStore() storage.ChainStore {
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

		// Periodic peer discovery is now handled by task scheduler

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
		s.peersMu.RLock()
		currentPeers := make(map[string]bool)
		for addr := range s.peers {
			currentPeers[addr] = true
		}
		s.peersMu.RUnlock()

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
		discoveredPeers := s.GetDiscoveredPeers()

		// Get current peers to avoid duplicate connections
		s.peersMu.RLock()
		currentPeers := make(map[string]bool)
		for addr := range s.peers {
			currentPeers[addr] = true
		}
		s.peersMu.RUnlock()

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

	// Get connected peers
	connectedPeers := s.GetConnectedPeers()
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
			newPeerCount := s.AddDiscoveredPeers(allDiscoveredPeers)
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
		s.peersMu.RLock()
		existingPeer, exists := s.peers[address]
		if exists && existingPeer.Status == PeerConnected {
			s.peersMu.RUnlock()
			s.logf("Already connected to peer %s, skipping connection attempt", address)
			result <- true // Already connected counts as success
			return
		}
		s.peersMu.RUnlock()

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
		actualPeer := s.AddPeer(address)
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
						s.peersMu.RLock()
						if peerObj, exists := s.peers[address]; exists && peerObj.Status == PeerConnected {
							s.peersMu.RUnlock()
							s.logf("Peer connection established: %s (may be incoming due to race)", address)
							handshakeComplete <- true
							return
						}
						s.peersMu.RUnlock()
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

// RunDiscoveryRound manually triggers one round of peer discovery
// Returns a channel that signals when the discovery round is complete
func (s *Server) RunDiscoveryRound() <-chan bool {
	done := make(chan bool, 1)

	go func() {
		defer func() { done <- true }()

		connected := s.GetConnectedPeers()
		var connectedAddrs []string
		for _, peer := range connected {
			connectedAddrs = append(connectedAddrs, peer.Address)
		}
		s.logf("Manual discovery check: %d connected peers: %v", len(connected), connectedAddrs)

		// Clean up dead peers
		removed := s.CleanupDeadPeers()
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


