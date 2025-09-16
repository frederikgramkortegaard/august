package networking

import (
	"august/blockchain"
	"august/config"
	"august/types"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// NodeInterface defines the interface that a node must implement for networking
type NodeInterface interface {
	ProcessBlock(block *blockchain.Block, excludePeer ...string) <-chan struct{}
	ProcessTransaction(tx *blockchain.Transaction, excludePeer ...string) error
	ProcessHeaders(headers []blockchain.BlockHeader, peer string) error
	ProcessNewBlockHeader(header *blockchain.BlockHeader, peer string) error
	ProcessChainHead(chainHead *ChainHeadPayload, peer string) error
	GetChainHeight() (uint64, error)
	GetChain() (*blockchain.Chain, error)
}

// Network handles network communication and message passing
type Network struct {
	networkConfig NetworkConfig
	listener      net.Listener

	// Reference to node for processing delegation
	node NodeInterface

	peers   map[string]Peer // Peers based on their address
	peersMu sync.RWMutex

	peerConnections   map[string]net.Conn // Active connections by peer address
	peerConnectionsMu sync.RWMutex        // Protects peerConnections map

	shutdown         chan bool // Signal to stop server
	shutdownComplete chan bool // Signal that server has stopped

	// Relay Tracking
	// Block relay tracking
	recentBlocks    map[types.Hash32]time.Time
	recentBlocksTTL time.Duration
	recentBlocksMu  sync.RWMutex

	// Transaction relay tracking
	recentTransactions    map[types.Hash32]time.Time
	recentTransactionsTTL time.Duration
	recentTransactionsMu  sync.RWMutex

	// Request-response tracking
	pendingRequests map[string]chan *Message // requestID -> response channel
	pendingMu       sync.RWMutex
}

// IsRecentTransaction checks if we've recently seen this transaction (to prevent relay loops)
func (n *Network) IsRecentTransaction(txHash types.Hash32) bool {
	n.recentTransactionsMu.RLock()
	defer n.recentTransactionsMu.RUnlock()

	if seenTime, exists := n.recentTransactions[txHash]; exists {
		// Check if it's still within TTL
		return time.Since(seenTime) < n.recentTransactionsTTL
	}
	return false
}

// MarkTransactionSeen records that we've seen this transaction
func (n *Network) MarkTransactionSeen(txHash types.Hash32) {
	n.recentTransactionsMu.Lock()
	defer n.recentTransactionsMu.Unlock()

	n.recentTransactions[txHash] = time.Now()
}

// NewNetwork creates a new network instance
func NewNetwork(networkConfig NetworkConfig, node NodeInterface) *Network {
	network := &Network{
		networkConfig: networkConfig,
		listener:      nil,  // Get's created on Network.Listen()
		node:          node, // Just a reference to the owner of this Network

		peers:           make(map[string]Peer),
		peerConnections: make(map[string]net.Conn),

		shutdown:         make(chan bool),
		shutdownComplete: make(chan bool),

		recentBlocks:    make(map[types.Hash32]time.Time),
		recentBlocksTTL: config.RecentBlocksTTL,

		recentTransactions:    make(map[types.Hash32]time.Time),
		recentTransactionsTTL: config.RecentTransactionsTTL,

		pendingRequests: make(map[string]chan *Message),
	}

	return network
}

// StartListener begins listening for network connections (TCP only)
func (n *Network) StartListener() error {
	listener, err := net.Listen("tcp", ":"+n.networkConfig.Port)
	if err != nil {
		return err
	}

	n.listener = listener
	fmt.Println("Network started listening")

	// Accept connections in background
	go n.acceptConnections()

	return nil
}

// acceptConnections handles incoming peer connections
func (n *Network) acceptConnections() {
	defer func() {
		n.shutdownComplete <- true
	}()

	for {
		// Set a short accept timeout to allow checking shutdown signal
		conn, err := n.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-n.shutdown:
				// Shutdown requested, exit gracefully without logging error
				return
			default:
				// Only log if it's not a shutdown-related error
				if !isNetworkClosedError(err) {
					fmt.Printf("Failed to accept connection: %v\n", err)

				}
				return
			}
		}

		// Check for shutdown before handling connection
		select {
		case <-n.shutdown:
			conn.Close()
			return
		default:
			// Handle peer connection in goroutine
			go n.HandlePeerConnection(conn)
		}
	}
}

// isNetworkClosedError checks if error is due to closed network connection
func isNetworkClosedError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}

// HandlePeerConnection manages communication with a connected peer (public for discovery)
func (n *Network) HandlePeerConnection(conn net.Conn) {
	defer conn.Close()

	connAddr := conn.RemoteAddr().String()
	fmt.Printf("New peer connection from: %s", connAddr)

	// For incoming connections, we don't know the peer's listening address yet
	// We'll get it from their handshake. For now, create a temporary peer entry
	peer := &Peer{
		Address:    connAddr, // Will be updated to listen address after handshake
		ConnAddr:   connAddr, // Actual connection endpoint
		Status:     PeerConnecting,
		IsOutgoing: false, // This is an incoming connection
	}

	// Store the connection using the connection address
	n.peerConnectionsMu.Lock()
	n.peerConnections[connAddr] = conn
	n.peerConnectionsMu.Unlock()

	defer func() {
		// Clean up the connection when done
		n.peerConnectionsMu.Lock()
		delete(n.peerConnections, connAddr)
		// Also try to delete by peer's actual address if it changed
		if peer.Address != connAddr {
			delete(n.peerConnections, peer.Address)
		}
		n.peerConnectionsMu.Unlock()

		// Mark peer as disconnected if it exists
		n.peersMu.Lock()
		if p, exists := n.peers[peer.Address]; exists {
			p.Status = PeerDisconnected
		}
		n.peersMu.Unlock()
	}()

	// Send handshake
	fmt.Printf("Sending handshake to %s", connAddr)
	n.sendHandshake(connAddr)

	// Handle incoming messages
	fmt.Printf("Starting message handler for %s", connAddr)
	n.handleMessages(conn, peer)
}

// sendHandshake sends initial handshake to a peer
func (n *Network) sendHandshake(peerAddr string) {
	height, err := n.node.GetChainHeight()
	if err != nil {
		fmt.Printf("Failed to get chain height: %v", err)
		height = 0
	}

	handshake := HandshakePayload{
		NodeID:      n.networkConfig.NodeID,
		ChainHeight: int(height),
		Version:     "1.0",
		ListenPort:  n.networkConfig.Port,
	}

	msg, err := NewMessage(MessageTypeHandshake, handshake)
	if err != nil {
		fmt.Printf("Failed to create handshake message: %v", err)
		return
	}

	fmt.Printf("Sending handshake message to %s (port: %s)", peerAddr, handshake.ListenPort)
	if err := n.SendNotification(peerAddr, msg); err != nil {
		fmt.Printf("Failed to send handshake: %v", err)
	}
}

// sendMessage sends a message over the connection
func (n *Network) sendMessage(conn net.Conn, msg *Message) error {
	encoder := json.NewEncoder(conn)
	err := encoder.Encode(msg)
	if err != nil {
		fmt.Printf("Failed to encode/send message %s: %v", msg.Type, err)
	}
	return err
}

// handleMessages processes incoming messages from a peer
func (n *Network) handleMessages(conn net.Conn, peer *Peer) {
	decoder := json.NewDecoder(conn)

	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				fmt.Printf("Peer %s disconnected", peer.Address)
				peer.Status = PeerDisconnected
			} else if strings.Contains(err.Error(), "connection reset") {
				fmt.Printf("Peer %s connection reset", peer.Address)
				peer.Status = PeerDisconnected
			} else {
				fmt.Printf("Error decoding message from peer %s: %v", peer.Address, err)
				peer.Status = PeerFailed
			}
			return
		}

		fmt.Printf("Received message type %s from %s", msg.Type, peer.Address)
		n.ProcessMessage(&msg, peer, conn)
	}
}

// GetListener returns the server listener (for testing)
func (n *Network) GetListener() net.Listener {
	return n.listener
}

// GetConnectedPeers returns all connected peers
func (n *Network) GetConnectedPeers() []*Peer {
	n.peersMu.RLock()
	defer n.peersMu.RUnlock()

	var connectedPeers []*Peer
	for _, peer := range n.peers {
		if peer.Status == PeerConnected {
			connectedPeers = append(connectedPeers, &peer)
		}
	}
	return connectedPeers
}

// Stop gracefully shuts down the network server
func (n *Network) Stop() error {
	if n.listener != nil {
		// Close the listener first
		n.listener.Close()
	}

	// Signal shutdown
	close(n.shutdown)

	// Wait for acceptConnections to finish
	<-n.shutdownComplete

	// Close all peer connections
	n.peerConnectionsMu.Lock()
	for addr, conn := range n.peerConnections {
		conn.Close()
		delete(n.peerConnections, addr)
	}
	n.peerConnectionsMu.Unlock()

	return nil
}

// StartDiscovery begins peer discovery process
func (n *Network) StartDiscovery() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		fmt.Printf("Starting peer discovery with %d seed peers", len(n.networkConfig.SeedPeers))

		// Connect to seed peers
		go n.connectToSeeds()

		// Start periodic tasks
		n.StartPeriodicDiscovery()
		n.StartPeriodicCleanup()

		fmt.Printf("Peer discovery started successfully")
		ready <- true
	}()

	return ready
}

// connectToSeeds attempts to connect to all seed peers and waits for completion
func (n *Network) connectToSeeds() <-chan bool {
	done := make(chan bool, 1)

	go func() {
		if len(n.networkConfig.SeedPeers) == 0 {
			done <- true
			return
		}

		// Get current peers to avoid duplicate connections
		n.peersMu.RLock()
		currentPeers := make(map[string]bool)
		for addr := range n.peers {
			currentPeers[addr] = true
		}
		n.peersMu.RUnlock()

		var connectionTasks []<-chan bool
		for _, seedPeer := range n.networkConfig.SeedPeers {
			if !currentPeers[seedPeer.Address] {
				connectionTasks = append(connectionTasks, n.ConnectToPeer(seedPeer.Address))
			} else {
				fmt.Printf("Skipping seed %s - already connected", seedPeer.Address)
			}
		}

		// Wait for all connection attempts to complete
		successCount := 0
		for _, task := range connectionTasks {
			if <-task {
				successCount++
			}
		}

		fmt.Printf("Seed connection attempts completed: %d/%d successful", successCount, len(n.networkConfig.SeedPeers))
		done <- true
	}()

	return done
}

// connectToDiscoveredPeers attempts to connect to peers we've learned about
func (n *Network) connectToDiscoveredPeers() <-chan bool {
	done := make(chan bool, 1)

	go func() {
		// TODO: Need to implement discovered peers tracking on Network
		// For now, just use empty list
		discoveredPeers := []string{}

		// Get current peers to avoid duplicate connections
		n.peersMu.RLock()
		currentPeers := make(map[string]bool)
		for addr := range n.peers {
			currentPeers[addr] = true
		}
		n.peersMu.RUnlock()

		// Try connecting to discovered peers we're not already connected to
		var connectionTasks []<-chan bool
		connected := 0
		maxConnections := config.MaxPeersForRequest
		for _, addr := range discoveredPeers {
			if !currentPeers[addr] && connected < maxConnections {
				connectionTasks = append(connectionTasks, n.ConnectToPeer(addr))
				connected++
			}
		}

		if connected > 0 {
			fmt.Printf("Attempting to connect to %d discovered peers", connected)
			// Wait for all connection attempts to complete
			successCount := 0
			for _, task := range connectionTasks {
				if <-task {
					successCount++
				}
			}
			fmt.Printf("Discovery connection attempts completed: %d/%d successful", successCount, connected)
		}

		done <- true
	}()

	return done
}

func (n *Network) requestPeerSharing() {
	// Get connected peers
	connectedPeers := n.GetConnectedPeers()
	if len(connectedPeers) == 0 {
		return
	}

	// Request peers from all connected peers
	successfulRequests := 0

	// Request peers from connected peers using existing message system
	for _, peer := range connectedPeers {
		// Create peer request message
		requestPayload := RequestPeersPayload{MaxPeers: 50}
		msg, err := NewMessage(MessageTypeRequestPeers, requestPayload)
		if err != nil {
			fmt.Printf("Failed to create peer request message: %v", err)
			continue
		}

		// Send request (this will be handled by the peer's SendPeerList method)
		if err := n.SendNotification(peer.Address, msg); err != nil {
			fmt.Printf("Failed to request peers from %s: %v", peer.Address, err)
		} else {
			successfulRequests++
		}
	}

	if successfulRequests > 0 {
		fmt.Printf("Successfully requested peers from %d peers", successfulRequests)
		// Note: Peer responses will be handled by ProcessSharedPeers in the message handler
	}
}

// requestPeerSharingAndConnect requests peers and then tries to connect to them
func (n *Network) requestPeerSharingAndConnect() <-chan bool {
	done := make(chan bool, 1)

	go func() {
		fmt.Printf("Starting peer sharing and connect sequence")
		// Request peers (synchronous - responses are handled immediately)
		n.requestPeerSharing()

		// Now try to connect to any newly discovered peers
		fmt.Printf("Now attempting to connect to discovered peers")
		<-n.connectToDiscoveredPeers()

		done <- true
	}()

	return done
}

// connectToPeer attempts to connect to a specific peer and waits for handshake completion
func (n *Network) ConnectToPeer(address string) <-chan bool {
	result := make(chan bool, 1)

	go func() {
		defer func() { result <- false }() // Default to failure

		// Check if we're already connected to this peer
		n.peersMu.RLock()
		existingPeer, exists := n.peers[address]
		if exists && existingPeer.Status == PeerConnected {
			n.peersMu.RUnlock()
			fmt.Printf("Already connected to peer %s, skipping connection attempt", address)
			result <- true // Already connected counts as success
			return
		}
		n.peersMu.RUnlock()

		fmt.Printf("Attempting to connect to peer: %s", address)

		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			fmt.Printf("Failed to connect to peer %s: %v", address, err)
			return
		}

		// Create peer entry for outgoing connection
		peer := &Peer{
			Address:    address,                   // Listen address (where we're connecting to)
			ConnAddr:   conn.LocalAddr().String(), // Our side of the connection
			Status:     PeerConnecting,
			IsOutgoing: true, // We initiated this connection
		}

		// Add peer directly to network
		n.peersMu.Lock()
		if existingPeer, exists := n.peers[address]; exists {
			// Update existing peer
			existingPeer.ConnAddr = conn.LocalAddr().String()
			existingPeer.IsOutgoing = true
			existingPeer.Status = PeerConnecting
			n.peers[address] = existingPeer
			peer = &existingPeer
		} else {
			// Create new peer
			newPeer := Peer{
				Address:    address,
				ConnAddr:   conn.LocalAddr().String(),
				Status:     PeerConnecting,
				IsOutgoing: true,
			}
			n.peers[address] = newPeer
			peer = &newPeer
		}
		n.peersMu.Unlock()

		fmt.Printf("TCP connection established to peer: %s", address)

		// For outgoing connections, we handle it differently
		// Store the connection
		n.peerConnectionsMu.Lock()
		n.peerConnections[address] = conn
		n.peerConnectionsMu.Unlock()

		// Send handshake immediately
		n.sendHandshake(address)

		// Start message handler (but NOT HandlePeerConnection as that's for incoming)
		go func() {
			defer conn.Close()
			defer func() {
				n.peerConnectionsMu.Lock()
				delete(n.peerConnections, address)
				n.peerConnectionsMu.Unlock()
				peer.Status = PeerDisconnected
			}()

			peer.Status = PeerConnected
			n.handleMessages(conn, peer)
		}()

		// Wait for handshake to complete using channels
		handshakeComplete := make(chan bool, 1)

		go func() {
			// Monitor for handshake completion
			timeout := time.NewTimer(10 * time.Second) // Increased timeout for race conditions
			defer timeout.Stop()

			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-timeout.C:
					fmt.Printf("Handshake timeout for peer %s", address)
					handshakeComplete <- false
					return
				case <-ticker.C:
					// Check if ANY connection to this peer exists (not just the one we initiated)
					n.peersMu.RLock()
					if peerObj, exists := n.peers[address]; exists && peerObj.Status == PeerConnected {
						n.peersMu.RUnlock()
						fmt.Printf("Peer connection established: %s (may be incoming due to race)", address)
						handshakeComplete <- true
						return
					}
					n.peersMu.RUnlock()
				}
			}
		}()

		// Wait for handshake result
		if <-handshakeComplete {
			result <- true
			return
		}
	}()

	return result
}

// RunDiscoveryRound manually triggers one round of peer discovery
// Returns a channel that signals when the discovery round is complete
func (n *Network) RunDiscoveryRound() <-chan bool {
	done := make(chan bool, 1)

	go func() {
		defer func() { done <- true }()

		connected := n.GetConnectedPeers()
		var connectedAddrs []string
		for _, peer := range connected {
			connectedAddrs = append(connectedAddrs, peer.Address)
		}
		fmt.Printf("Manual discovery check: %d connected peers: %v", len(connected), connectedAddrs)

		// Clean up dead peers - implement this directly
		removed := n.cleanupDeadPeers()
		if removed > 0 {
			fmt.Printf("Cleaned up %d dead peers", removed)
		}

		// Different strategies based on connection count
		if len(connected) == 0 {
			// No connections, try seed peers
			fmt.Printf("No connected peers, attempting to connect to seed peers")
			<-n.connectToSeeds() // Wait for seed connections to complete
		} else if len(connected) < 15 {
			// Few connections, try to get more
			<-n.connectToDiscoveredPeers()
			<-n.requestPeerSharingAndConnect()
		} else if len(connected) < 20 {
			<-n.connectToDiscoveredPeers()
		}

		fmt.Printf("Discovery round completed")
	}()

	return done
}

// shouldKeepExistingConnection determines which connection to keep when duplicates are detected
// Uses a deterministic rule based on node IDs to ensure both nodes make the same decision
func (n *Network) shouldKeepExistingConnection(existingPeer *Peer, newPeer *Peer, remoteNodeID string) bool {
	// Use lexicographic comparison of node IDs to make consistent decisions across nodes
	// The node with the smaller ID keeps its outgoing connection
	localNodeID := n.networkConfig.NodeID

	if localNodeID < remoteNodeID {
		// We have the smaller ID, so we keep our outgoing connections
		return existingPeer.IsOutgoing
	} else {
		// Remote has the smaller ID, so we keep incoming connections (their outgoing)
		return !existingPeer.IsOutgoing
	}
}

// selectPeersForRequest implements Bitcoin-style peer selection with randomization and limits
func (n *Network) selectPeersForRequest() []*Peer {
	// Get all connected peers
	availablePeers := n.GetConnectedPeers()
	if len(availablePeers) == 0 {
		return nil
	}

	// Start with a copy to avoid modifying the original slice
	candidates := make([]*Peer, len(availablePeers))
	copy(candidates, availablePeers)

	// Randomize peer order if configured
	if config.RandomizePeerOrder {
		// TODO: Add math/rand import
		for i := len(candidates) - 1; i > 0; i-- {
			j := i // simplified for now
			candidates[i], candidates[j] = candidates[j], candidates[i]
		}
	}

	// TODO: Add peer performance/responsiveness tracking here
	// For now, just respect the configured limits

	maxPeers := config.MaxPeersForRequest
	if len(candidates) < maxPeers {
		maxPeers = len(candidates)
	}

	// Ensure we have at least the minimum required peers
	if maxPeers < config.MinPeersForRequest && len(candidates) >= config.MinPeersForRequest {
		maxPeers = config.MinPeersForRequest
	}

	fmt.Printf("Selected %d peers for request from %d available (randomized: %v)\n",
		maxPeers, len(availablePeers), config.RandomizePeerOrder)

	return candidates[:maxPeers]
}
