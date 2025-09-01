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
	orphanPool        map[blockchain.Hash32]*blockchain.Block // Orphan blocks waiting for parents
	orphanPoolMu      sync.RWMutex                            // Protects orphan pool
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
		orphanPool:       make(map[blockchain.Hash32]*blockchain.Block),
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
