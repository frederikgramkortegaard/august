package p2p

import (
	"log"
	"net"
	"time"
)

// DiscoveryConfig holds configuration for peer discovery
type DiscoveryConfig struct {
	SeedPeers []string
	P2PServer *Server
}

// Discovery handles finding and connecting to peers
type Discovery struct {
	config DiscoveryConfig
}

// NewDiscovery creates a new discovery service
func NewDiscovery(config DiscoveryConfig) *Discovery {
	return &Discovery{
		config: config,
	}
}

// Start begins peer discovery process
func (d *Discovery) Start() {
	log.Printf("Starting peer discovery with %d seed peers", len(d.config.SeedPeers))

	// Connect to seed peers
	go d.connectToSeeds()

	// Periodic peer discovery
	go d.periodicDiscovery()
}

// connectToSeeds attempts to connect to all seed peers
func (d *Discovery) connectToSeeds() {
	for _, seedAddr := range d.config.SeedPeers {
		go d.connectToPeer(seedAddr)
	}
}

// connectToPeer attempts to connect to a specific peer
func (d *Discovery) connectToPeer(address string) {
	log.Printf("Attempting to connect to peer: %s", address)

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		log.Printf("Failed to connect to peer %s: %v", address, err)
		return
	}

	// Add peer to our peer manager (but don't mark as connected yet)
	if d.config.P2PServer != nil {
		peer := d.config.P2PServer.GetPeerManager().AddPeer(address)
		if peer != nil {
			log.Printf("Successfully connected to peer: %s", address)
			// Hand over connection to P2P server for proper management
			go d.config.P2PServer.HandlePeerConnection(conn)
		} else {
			// If we can't add the peer, close the connection
			conn.Close()
		}
	} else {
		conn.Close()
	}
}

// periodicDiscovery runs periodic peer discovery
func (d *Discovery) periodicDiscovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Check if P2P server is available
		if d.config.P2PServer == nil {
			log.Printf("P2P server not available, skipping periodic discovery")
			continue
		}

		connected := d.config.P2PServer.GetPeerManager().GetConnectedPeers()
		log.Printf("Periodic discovery check: %d connected peers", len(connected))

		// If we have few peers, try to connect to seeds again
		if len(connected) < 2 {
			go d.connectToSeeds()
		}
	}
}
