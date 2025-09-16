package types

import (
	"net"
	"sync"
	"time"
)

type PeerStatus int

const (
	PeerDisconnected PeerStatus = iota
	PeerConnecting
	PeerConnected
	PeerFailed
)

type Peer struct {
	ID         string
	Address    string // Listen address (where peer accepts connections)
	ConnAddr   string // Connection address (actual TCP endpoint, might be ephemeral)
	LastSeen   time.Time
	Status     PeerStatus
	IsOutgoing bool // True if we initiated the connection
}

// NetworkConfig holds network configuration
type NetworkConfig struct {
	Port      string
	NodeID    string
	SeedPeers []Peer // Seed peers for discovery
}

// Network handles network communication and message passing
type Network struct {
	config   NetworkConfig
	listener net.Listener

	// Reference to node for processing delegation
	node interface {
		ProcessBlock(block *Block, excludePeer ...string) <-chan struct{}
		ProcessTransaction(tx *Transaction, excludePeer ...string) error
		ProcessHeaders(headers []BlockHeader, peer string) error
		ProcessNewBlockHeader(header *BlockHeader, peer string) error
		ProcessChainHead(chainHead *ChainHeadPayload, peer string) error
	}

	peers   map[string]Peer // Peers based on their address
	peersMu sync.RWMutex

	peerConnections   map[string]net.Conn // Active connections by peer address
	peerConnectionsMu sync.RWMutex        // Protects peerConnections map

	shutdown         chan bool // Signal to stop server
	shutdownComplete chan bool // Signal that server has stopped

	// Relay Tracking
	// // Block relay tracking
	recentBlocks    map[Hash32]time.Time
	recentBlocksTTL time.Duration
	recentBlocksMu  sync.RWMutex

	// // Transaction relay tracking
	recentTransactions    map[Hash32]time.Time
	recentTransactionsTTL time.Duration
	recentTransactionsMu  sync.RWMutex

	// Request-response tracking
	pendingRequests map[string]chan *Message // requestID -> response channel
	pendingMu       sync.RWMutex
}
