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
func (d *Discovery) Start() <-chan bool {
	ready := make(chan bool, 1)
	
	go func() {
		d.logf("Starting peer discovery with %d seed peers", len(d.config.SeedPeers))

		// Double-check P2P server is available
		if d.config.P2PServer == nil {
			d.logf("P2P server not available, discovery cannot start")
			ready <- false
			return
		}

		// Connect to seed peers
		go d.connectToSeeds()

		// Periodic peer discovery
		go d.periodicDiscovery()
		
		d.logf("Peer discovery started successfully")
		ready <- true
	}()
	
	return ready
}

// connectToSeeds attempts to connect to all seed peers and waits for completion
func (d *Discovery) connectToSeeds() <-chan bool {
	done := make(chan bool, 1)
	
	go func() {
		if len(d.config.SeedPeers) == 0 {
			done <- true
			return
		}
		
		var connectionTasks []<-chan bool
		for _, seedAddr := range d.config.SeedPeers {
			connectionTasks = append(connectionTasks, d.connectToPeer(seedAddr))
		}
		
		// Wait for all connection attempts to complete
		successCount := 0
		for _, task := range connectionTasks {
			if <-task {
				successCount++
			}
		}
		
		d.logf("Seed connection attempts completed: %d/%d successful", successCount, len(d.config.SeedPeers))
		done <- true
	}()
	
	return done
}

// connectToDiscoveredPeers attempts to connect to peers we've learned about
func (d *Discovery) connectToDiscoveredPeers() <-chan bool {
	done := make(chan bool, 1)
	
	go func() {
		if d.config.P2PServer == nil {
			done <- true
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
		var connectionTasks []<-chan bool
		connected := 0
		for _, addr := range discoveredPeers {
			if !currentPeers[addr] && connected < 5 { // Limit to 5 new connections per cycle
				connectionTasks = append(connectionTasks, d.connectToPeer(addr))
				connected++
			}
		}
		
		if connected > 0 {
			d.logf("Attempting to connect to %d discovered peers", connected)
			// Wait for all connection attempts to complete
			successCount := 0
			for _, task := range connectionTasks {
				if <-task {
					successCount++
				}
			}
			d.logf("Discovery connection attempts completed: %d/%d successful", successCount, connected)
		}
		
		done <- true
	}()
	
	return done
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

	// Request peers from all connected peers and collect responses
	allDiscoveredPeers := make([]string, 0)
	successfulRequests := 0
	
	for _, peer := range connectedPeers {
		if peer.Status == PeerConnected {
			// Use the new synchronous method to get immediate response
			peers, err := RequestPeersFromPeer(d.config.P2PServer, peer.Address, 50)
			if err != nil {
				d.logf("Failed to request peers from %s: %v", peer.Address, err)
			} else {
				allDiscoveredPeers = append(allDiscoveredPeers, peers...)
				successfulRequests++
			}
		}
	}
	
	if successfulRequests > 0 {
		d.logf("Successfully requested peers from %d peers", successfulRequests)
		
		// Add all discovered peers to our peer manager
		if len(allDiscoveredPeers) > 0 {
			newPeerCount := pm.AddDiscoveredPeers(allDiscoveredPeers)
			if newPeerCount > 0 {
				d.logf("Discovered %d new peers through peer sharing", newPeerCount)
			}
		}
	}
}

// requestPeerSharingAndConnect requests peers and then tries to connect to them
func (d *Discovery) requestPeerSharingAndConnect() <-chan bool {
	done := make(chan bool, 1)
	
	go func() {
		d.logf("Starting peer sharing and connect sequence")
		// Request peers (synchronous - responses are handled immediately)
		d.requestPeerSharing()
		
		// Now try to connect to any newly discovered peers
		d.logf("Now attempting to connect to discovered peers")
		<-d.connectToDiscoveredPeers()
		
		done <- true
	}()
	
	return done
}

// connectToPeer attempts to connect to a specific peer and waits for handshake completion
func (d *Discovery) connectToPeer(address string) <-chan bool {
	result := make(chan bool, 1)
	
	go func() {
		defer func() { result <- false }() // Default to failure
		
		d.logf("Attempting to connect to peer: %s", address)

		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			d.logf("Failed to connect to peer %s: %v", address, err)
			return
		}

		// Add peer to our peer manager
		if d.config.P2PServer != nil {
			peer := d.config.P2PServer.GetPeerManager().AddPeer(address)
			if peer != nil {
				d.logf("TCP connection established to peer: %s", address)
				
				// Start the connection handler in background
				go d.config.P2PServer.HandlePeerConnection(conn)
				
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
							d.logf("Handshake timeout for peer %s", address)
							handshakeComplete <- false
							return
						case <-ticker.C:
							// Check if peer is now connected (handshake completed)
							pm := d.config.P2PServer.GetPeerManager()
							pm.mu.RLock()
							if peerObj, exists := pm.peers[address]; exists && peerObj.Status == PeerConnected {
								pm.mu.RUnlock()
								d.logf("Handshake completed with peer: %s", address)
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
		} else {
			conn.Close()
		}
	}()
	
	return result
}

// periodicDiscovery runs periodic peer discovery
func (d *Discovery) periodicDiscovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Just trigger a manual discovery round
		<-d.RunDiscoveryRound()
	}
}

// RunDiscoveryRound manually triggers one round of peer discovery
// Returns a channel that signals when the discovery round is complete
func (d *Discovery) RunDiscoveryRound() <-chan bool {
	done := make(chan bool, 1)
	
	go func() {
		defer func() { done <- true }()
		
		// Check if P2P server is available
		if d.config.P2PServer == nil {
			d.logf("P2P server not available, skipping discovery")
			return
		}

		pm := d.config.P2PServer.GetPeerManager()
		connected := pm.GetConnectedPeers()
		var connectedAddrs []string
		for _, peer := range connected {
			connectedAddrs = append(connectedAddrs, peer.Address)
		}
		d.logf("Manual discovery check: %d connected peers: %v", len(connected), connectedAddrs)

		// Clean up dead peers
		removed := pm.CleanupDeadPeers()
		if removed > 0 {
			d.logf("Cleaned up %d dead peers", removed)
		}
		
		// Different strategies based on connection count
		if len(connected) == 0 {
			// No connections, try seed peers
			d.logf("No connected peers, attempting to connect to seed peers")
			<-d.connectToSeeds() // Wait for seed connections to complete
		} else if len(connected) < 5 {
			// Few connections, try to get more
			<-d.connectToDiscoveredPeers()
			<-d.requestPeerSharingAndConnect()
		} else if len(connected) < 10 {
			<-d.connectToDiscoveredPeers()
		}
		
		d.logf("Discovery round completed")
	}()
	
	return done
}
