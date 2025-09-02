package p2p

import (
	"encoding/json"
	"fmt"
	"august/blockchain"
	"august/blockchain/store"
	"august/p2p/reqresp"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds P2P server configuration
type Config struct {
	Port          string
	NodeID        string
	Store         store.ChainStore
	ReqRespConfig reqresp.Config // Request-response configuration
}

// DefaultReqRespConfig returns default request-response configuration
func DefaultReqRespConfig() reqresp.Config {
	return reqresp.DefaultConfig()
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

// NewServer creates a new P2P server
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
