package config

import "time"

// Networking configuration constants
const (
	DefaultPort      = "8333"
	DefaultNodeID    = "august-node"
	MaxPeers         = 125
	MaxPendingPeers  = 25
	ConnectionTimeout = 30 * time.Second

	// Request-response
	RequestTimeout     = 5 * time.Second
	MaxPendingRequests = 100

	// Relay tracking
	RecentBlocksTTL       = 5 * time.Minute
	RecentTransactionsTTL = 5 * time.Minute
	CleanupInterval       = 30 * time.Second

	// Keepalive
	PingInterval = 30 * time.Second
	PongTimeout  = 60 * time.Second

	// Message sizes
	MaxMessageSize = 32 * 1024 * 1024 // 32 MB max message size
	MaxHeadersInBatch = 2000           // Max headers in one message
	MaxBlocksInBatch  = 500            // Max blocks in one message

	// Peer request configuration (Bitcoin-style)
	MaxRequestsPerPeer    = 16   // Conservative like Bitcoin Core
	MaxPeersForRequest    = 20   // Try up to 20 peers in parallel/sequence
	MinPeersForRequest    = 1    // Can sync from 1 peer (like Bitcoin)
	RequestTimeoutSec     = 30   // 30 second timeout per peer
	RandomizePeerOrder    = true // Randomize peer selection
	PreferResponsivePeers = true // Prefer historically responsive peers
)