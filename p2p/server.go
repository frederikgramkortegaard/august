package p2p

import (
	"encoding/json"
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"io"
	"log"
	"net"
	"strings"
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
}

// Server handles P2P networking and message passing
type Server struct {
	config      Config
	listener    net.Listener
	peerManager *PeerManager
}

// NewServer creates a new P2P server
func NewServer(config Config) *Server {
	return &Server{
		config:      config,
		peerManager: NewPeerManager([]string{}), // Will be set by discovery
	}
}

// Start begins listening for P2P connections
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", ":"+s.config.Port)
	if err != nil {
		return err
	}

	s.listener = listener
	log.Printf("P2P server listening on port %s", s.config.Port)

	// Accept connections in background
	go s.acceptConnections()

	return nil
}

// acceptConnections handles incoming peer connections
func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		// Handle peer connection in goroutine
		go s.handlePeerConnection(conn)
	}
}

// handlePeerConnection manages communication with a connected peer
func (s *Server) handlePeerConnection(conn net.Conn) {
	defer conn.Close()

	peerAddr := conn.RemoteAddr().String()
	log.Printf("New peer connection from: %s", peerAddr)

	peer := s.peerManager.AddPeer(peerAddr)
	if peer == nil {
		log.Printf("Failed to add peer %s (max peers reached)", peerAddr)
		return
	}

	peer.Status = PeerConnected
	log.Printf("Peer %s connected", peerAddr)

	// Send handshake
	s.sendHandshake(conn)

	// Handle incoming messages
	s.handleMessages(conn, peer)
}

// sendHandshake sends initial handshake to a peer
func (s *Server) sendHandshake(conn net.Conn) {
	height, err := s.config.Store.GetChainHeight()
	if err != nil {
		log.Printf("Failed to get chain height: %v", err)
		height = 0
	}

	handshake := HandshakePayload{
		NodeID:      s.config.NodeID,
		ChainHeight: int(height),
		Version:     "1.0",
	}

	msg, err := NewMessage(MessageTypeHandshake, handshake)
	if err != nil {
		log.Printf("Failed to create handshake message: %v", err)
		return
	}

	s.sendMessage(conn, msg)
}

// sendMessage sends a message over the connection
func (s *Server) sendMessage(conn net.Conn, msg *Message) error {
	encoder := json.NewEncoder(conn)
	return encoder.Encode(msg)
}

// handleMessages processes incoming messages from a peer
func (s *Server) handleMessages(conn net.Conn, peer *Peer) {
	decoder := json.NewDecoder(conn)

	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				log.Printf("Peer %s disconnected", peer.Address)
				peer.Status = PeerDisconnected
			} else if strings.Contains(err.Error(), "connection reset") {
				log.Printf("Peer %s connection reset", peer.Address)
				peer.Status = PeerDisconnected
			} else {
				log.Printf("Error decoding message from peer %s: %v", peer.Address, err)
				peer.Status = PeerFailed
			}
			return
		}

		s.processMessage(&msg, peer, conn)
	}
}

// processMessage handles different types of P2P messages
func (s *Server) processMessage(msg *Message, peer *Peer, conn net.Conn) {
	switch msg.Type {
	case MessageTypeHandshake:
		var handshake HandshakePayload
		if err := msg.ParsePayload(&handshake); err != nil {
			log.Printf("Failed to parse handshake: %v", err)
			return
		}
		log.Printf("Received handshake from %s (height: %d)", handshake.NodeID, handshake.ChainHeight)

	case MessageTypeNewBlock:
		var blockPayload NewBlockPayload
		if err := msg.ParsePayload(&blockPayload); err != nil {
			log.Printf("Failed to parse new block: %v", err)
			return
		}
		blockHash := blockchain.HashBlockHeader(&blockPayload.Block.Header)
		log.Printf("Received new block %x from peer %s", blockHash[:8], peer.Address)
		
		// Process block using the FullNode's ProcessBlock method (handles orphans)
		if s.config.BlockProcessor != nil {
			if err := s.config.BlockProcessor.ProcessBlock(blockPayload.Block); err != nil {
				log.Printf("Failed to process block %x from peer %s: %v", blockHash[:8], peer.Address, err)
			} else {
				log.Printf("Successfully processed block %x from peer %s", blockHash[:8], peer.Address)
			}
		} else {
			log.Printf("No block processor configured, cannot process block %x", blockHash[:8])
		}

	case MessageTypePing:
		// Respond with pong
		pong := PongPayload{Timestamp: time.Now().Unix()}
		if pongMsg, err := NewMessage(MessageTypePong, pong); err == nil {
			s.sendMessage(conn, pongMsg)
		}

	case MessageTypePong:
		log.Printf("Received pong from peer %s", peer.Address)

	default:
		log.Printf("Unknown message type: %s", msg.Type)
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
