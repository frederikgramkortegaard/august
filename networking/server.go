package networking

import (
	"august/blockchain"
	store "august/storage"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds network server configuration
type Config struct {
	Port          string
	NodeID        string
	Store         store.ChainStore
	SeedPeers     []string        // Seed peers for discovery
	ReqRespConfig ReqRespConfig   // Request-response configuration
}


// CandidateChain represents a potential blockchain candidate for adoption
type CandidateChain struct {
	ID           string
	PeerSource   string
	ChainStore   store.ChainStore
	Headers      []blockchain.BlockHeader
	StartedAt    time.Time
	ExpectedWork string

	// Atomic progress tracking
	expectedHeight atomic.Uint64
	currentHeight  atomic.Uint64
	downloadStatus atomic.Uint32 // 0=downloading, 1=complete, 2=failed
}

// DownloadStatus returns the current download status (for testing)
func (c *CandidateChain) DownloadStatus() *atomic.Uint32 {
	return &c.downloadStatus
}

// CandidateBlock represents a single block waiting for context or validation
type CandidateBlock struct {
	Block        *blockchain.Block
	Source       string
	ReceivedAt   time.Time
	ParentNeeded blockchain.Hash32
}

// Server handles network communication and message passing
type Server struct {
	config            Config
	listener          net.Listener
	peerManager       *PeerManager
	peerConnections   map[string]net.Conn // Active connections by peer address
	peerConnectionsMu sync.RWMutex        // Protects peerConnections map
	shutdown          chan bool           // Signal to stop server
	shutdownComplete  chan bool           // Signal that server has stopped
	reqRespClient     *ReqRespClient     // Request-response client
	recentBlocks      map[blockchain.Hash32]time.Time
	recentBlocksTTL   time.Duration
	recentBlocksMu    sync.RWMutex

	// New candidate chain system (lock-free)
	candidateChains sync.Map // map[string]*CandidateChain
	candidateBlocks sync.Map // map[blockchain.Hash32]*CandidateBlock

	// Unified cleanup system
	cleanupTicker   *time.Ticker
	cleanupInterval time.Duration
}

// logf logs with node ID prefix
func (s *Server) logf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	log.Printf("%s\t%s", s.config.NodeID, message)
}

// NewServer creates a new network server
func NewServer(config Config) *Server {
	server := &Server{
		config:           config,
		peerManager:      NewPeerManager([]string{}), // Will be set by discovery
		peerConnections:  make(map[string]net.Conn),
		shutdown:         make(chan bool),
		shutdownComplete: make(chan bool),
		recentBlocks:     make(map[blockchain.Hash32]time.Time),
		recentBlocksTTL:  5 * time.Minute,
		cleanupInterval:  30 * time.Second,
		// candidateChains and candidateBlocks are sync.Maps - no initialization needed
	}

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

// Start begins listening for network connections
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", ":"+s.config.Port)
	if err != nil {
		return err
	}

	s.listener = listener
	s.logf("Network server listening on port %s", s.config.Port)

	// Accept connections in background
	go s.acceptConnections()

	// Start unified periodic cleanup system
	s.cleanupTicker = time.NewTicker(s.cleanupInterval)
	go s.unifiedPeriodicCleanup()

	go s.periodicChainSync()

	return nil
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

	// 2. Clean up failed/old candidate chains
	s.cleanCandidateChains(now)

	// 3. Clean up candidate blocks (replaces orphan pool)
	s.cleanCandidateBlocks(now)

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
	blockHash := blockchain.HashBlockHeader(&latestBlock.Header)
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

func (s *Server) cleanCandidateChains(now time.Time) {
	var toDelete []string

	s.candidateChains.Range(func(key, value interface{}) bool {
		candidate := value.(*CandidateChain)
		candidateID := key.(string)

		shouldDelete := false

		// Delete if failed
		if candidate.downloadStatus.Load() == 2 {
			shouldDelete = true
		}

		// Delete if too old (prevent memory leaks)
		if now.Sub(candidate.StartedAt) > 10*time.Minute {
			shouldDelete = true
		}

		if shouldDelete {
			toDelete = append(toDelete, candidateID)
		}

		return true
	})

	for _, id := range toDelete {
		s.candidateChains.Delete(id)
	}

	if len(toDelete) > 0 {
		s.logf("Cleaned up %d old candidate chains", len(toDelete))
	}
}

func (s *Server) cleanCandidateBlocks(now time.Time) {
	var toDelete []blockchain.Hash32

	s.candidateBlocks.Range(func(key, value interface{}) bool {
		blockHash := key.(blockchain.Hash32)
		candidateBlock := value.(*CandidateBlock)

		// Delete blocks older than 5 minutes
		if now.Sub(candidateBlock.ReceivedAt) > 5*time.Minute {
			toDelete = append(toDelete, blockHash)
		}

		return true
	})

	for _, hash := range toDelete {
		s.candidateBlocks.Delete(hash)
	}

	if len(toDelete) > 0 {
		s.logf("Cleaned up %d old candidate blocks", len(toDelete))
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
	s.sendHandshake(peerAddr)

	// Handle incoming messages
	s.logf("Starting message handler for %s", peerAddr)
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

// GetCandidateChains returns the candidate chains map (for testing)
func (s *Server) GetCandidateChains() *sync.Map {
	return &s.candidateChains
}

// GetCandidateBlocks returns the candidate blocks map (for testing)
func (s *Server) GetCandidateBlocks() *sync.Map {
	return &s.candidateBlocks
}

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

		var connectionTasks []<-chan bool
		for _, seedAddr := range s.config.SeedPeers {
			connectionTasks = append(connectionTasks, s.connectToPeer(seedAddr))
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
		for _, addr := range discoveredPeers {
			if !currentPeers[addr] && connected < 5 { // Limit to 5 new connections per cycle
				connectionTasks = append(connectionTasks, s.connectToPeer(addr))
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

	for _, peer := range connectedPeers {
		if peer.Status == PeerConnected {
			// Use the new synchronous method to get immediate response
			peers, err := RequestPeersFromPeer(s, peer.Address, 50)
			if err != nil {
				s.logf("Failed to request peers from %s: %v", peer.Address, err)
			} else {
				allDiscoveredPeers = append(allDiscoveredPeers, peers...)
				successfulRequests++
			}
		}
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
func (s *Server) connectToPeer(address string) <-chan bool {
	result := make(chan bool, 1)

	go func() {
		defer func() { result <- false }() // Default to failure

		s.logf("Attempting to connect to peer: %s", address)

		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			s.logf("Failed to connect to peer %s: %v", address, err)
			return
		}

		// Add peer to our peer manager
		peer := s.GetPeerManager().AddPeer(address)
		if peer != nil {
			s.logf("TCP connection established to peer: %s", address)

			// Start the connection handler in background
			go s.HandlePeerConnection(conn)

			// Wait for handshake to complete using channels
			handshakeComplete := make(chan bool, 1)

			go func() {
				// Monitor for handshake completion
				timeout := time.NewTimer(5 * time.Second)
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
						// Check if peer is now connected (handshake completed)
						pm := s.GetPeerManager()
						pm.mu.RLock()
						if peerObj, exists := pm.peers[address]; exists && peerObj.Status == PeerConnected {
							pm.mu.RUnlock()
							s.logf("Handshake completed with peer: %s", address)
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
		} else if len(connected) < 5 {
			// Few connections, try to get more
			<-s.connectToDiscoveredPeers()
			<-s.requestPeerSharingAndConnect()
		} else if len(connected) < 10 {
			<-s.connectToDiscoveredPeers()
		}

		s.logf("Discovery round completed")
	}()

	return done
}
