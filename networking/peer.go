package networking

import "time"

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
