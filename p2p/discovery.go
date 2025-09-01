package p2p

import (
	"fmt"
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

// logf logs with node ID prefix
func (d *Discovery) logf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if d.config.P2PServer != nil {
		log.Printf("%s\t%s", d.config.P2PServer.config.NodeID, message)
	} else {
		log.Printf("DISCOVERY\t%s", message)
	}
}

// NewDiscovery creates a new discovery service
func NewDiscovery(config DiscoveryConfig) *Discovery {
	return &Discovery{
		config: config,
	}
}

// Start begins peer discovery process
func (d *Discovery) Start() {
	d.logf("Starting peer discovery with %d seed peers", len(d.config.SeedPeers))

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

// connectToDiscoveredPeers attempts to connect to peers we've learned about
func (d *Discovery) connectToDiscoveredPeers() {
	if d.config.P2PServer == nil {
		return
	}
	
	pm := d.config.P2PServer.GetPeerManager()
	discoveredPeers := pm.GetDiscoveredPeers()
	
	// Get current peers to avoid duplicate connections
	pm.mu.RLock()
	currentPeers := make(map[string]bool)
	for addr := range pm.peers {
		currentPeers[addr] = true
	}
	pm.mu.RUnlock()
	
	// Try connecting to discovered peers we're not already connected to
	connected := 0
	for _, addr := range discoveredPeers {
		if !currentPeers[addr] && connected < 5 { // Limit to 5 new connections per cycle
			go d.connectToPeer(addr)
			connected++
		}
	}
	
	if connected > 0 {
		d.logf("Attempting to connect to %d discovered peers", connected)
	}
}

func (d *Discovery) requestPeerSharing() {
	pm := d.config.P2PServer.GetPeerManager()
	if pm == nil {
		return
	}

	// Get connected peers
	connectedPeers := pm.GetConnectedPeers()
	if len(connectedPeers) == 0 {
		return
	}

	// Send peer requests to all connected peers
	requestsSent := 0
	for _, peer := range connectedPeers {
		if peer.Status == PeerConnected {
			// Send request for up to 50 peers from each connected peer
			err := d.config.P2PServer.SendPeerRequest(peer.Address, 50)
			if err != nil {
				d.logf("Failed to request peers from %s: %v", peer.Address, err)
			} else {
				requestsSent++
			}
		}
	}
	
	if requestsSent > 0 {
		d.logf("Sent peer requests to %d connected peers", requestsSent)
	}
	
	// The responses will be handled by handleMessages in server.go
	// which will call AddDiscoveredPeers directly

}

// connectToPeer attempts to connect to a specific peer
func (d *Discovery) connectToPeer(address string) {
	d.logf("Attempting to connect to peer: %s", address)

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		d.logf("Failed to connect to peer %s: %v", address, err)
		return
	}

	// Add peer to our peer manager (but don't mark as connected yet)
	if d.config.P2PServer != nil {
		peer := d.config.P2PServer.GetPeerManager().AddPeer(address)
		if peer != nil {
			d.logf("Successfully connected to peer: %s", address)
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
			d.logf("P2P server not available, skipping periodic discovery")
			continue
		}

		pm := d.config.P2PServer.GetPeerManager()
		connected := pm.GetConnectedPeers()
		d.logf("Periodic discovery check: %d connected peers", len(connected))

		// Clean up dead peers
		removed := pm.CleanupDeadPeers()
		if removed > 0 {
			d.logf("Cleaned up %d dead peers", removed)
		}

		// Different strategies based on connection count
		if len(connected) == 0 {
			// No connections, try seed peers
			d.logf("No connected peers, attempting to connect to seed peers")
			go d.connectToSeeds()
		} else if len(connected) < 5 {
			// Few connections, try to get more
			go d.requestPeerSharing()
			go d.connectToDiscoveredPeers()
		} else {
			// Good number of connections, just maintain
			if len(connected) < 10 {
				go d.connectToDiscoveredPeers()
			}
			// Always try to discover new peers
			go d.requestPeerSharing()
		}
	}
}
