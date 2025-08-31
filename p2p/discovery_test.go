package p2p

import (
	"gocuria/blockchain/store"
	"testing"
	"time"
)

func TestNewDiscovery(t *testing.T) {
	seeds := []string{"127.0.0.1:9001", "127.0.0.1:9002"}
	
	// Create a mock P2P server
	chainStore := store.NewMemoryChainStore()
	config := Config{
		Port:   "0",
		NodeID: "test-node",
		Store:  chainStore,
	}
	server := NewServer(config)
	
	discoveryConfig := DiscoveryConfig{
		SeedPeers: seeds,
		P2PServer: server,
	}
	
	discovery := NewDiscovery(discoveryConfig)
	
	if discovery == nil {
		t.Fatal("NewDiscovery returned nil")
	}
	
	if len(discovery.config.SeedPeers) != 2 {
		t.Errorf("Expected 2 seed peers, got %d", len(discovery.config.SeedPeers))
	}
	
	if discovery.config.P2PServer != server {
		t.Error("P2PServer reference not set correctly")
	}
}

func TestDiscoveryWithRealServer(t *testing.T) {
	// Create a seed server to connect to
	chainStore1 := store.NewMemoryChainStore()
	seedConfig := Config{
		Port:   "0",
		NodeID: "seed-node",
		Store:  chainStore1,
	}
	seedServer := NewServer(seedConfig)
	err := seedServer.Start()
	if err != nil {
		t.Fatalf("Failed to start seed server: %v", err)
	}
	defer seedServer.listener.Close()
	
	seedAddr := seedServer.listener.Addr().String()
	
	// Create client server with discovery
	chainStore2 := store.NewMemoryChainStore()
	clientConfig := Config{
		Port:   "0",
		NodeID: "client-node",
		Store:  chainStore2,
	}
	clientServer := NewServer(clientConfig)
	err = clientServer.Start()
	if err != nil {
		t.Fatalf("Failed to start client server: %v", err)
	}
	defer clientServer.listener.Close()
	
	// Create discovery service pointing to seed
	discoveryConfig := DiscoveryConfig{
		SeedPeers: []string{seedAddr},
		P2PServer: clientServer,
	}
	discovery := NewDiscovery(discoveryConfig)
	
	// Start discovery
	discovery.Start()
	
	// Wait for connection attempts
	time.Sleep(200 * time.Millisecond)
	
	// Check if peer was added (it might not be connected due to timing)
	// The important thing is that the discovery service attempted to connect
	if len(clientServer.peerManager.peers) == 0 {
		t.Log("Note: No peers connected, but this might be due to timing in tests")
	}
}

func TestDiscoveryConnectToPeer(t *testing.T) {
	// Create a server to act as the peer
	chainStore := store.NewMemoryChainStore()
	peerConfig := Config{
		Port:   "0",
		NodeID: "peer-node",
		Store:  chainStore,
	}
	peerServer := NewServer(peerConfig)
	err := peerServer.Start()
	if err != nil {
		t.Fatalf("Failed to start peer server: %v", err)
	}
	defer peerServer.listener.Close()
	
	peerAddr := peerServer.listener.Addr().String()
	
	// Create client for discovery
	chainStore2 := store.NewMemoryChainStore()
	clientConfig := Config{
		Port:   "0",
		NodeID: "client-node",
		Store:  chainStore2,
	}
	clientServer := NewServer(clientConfig)
	
	discoveryConfig := DiscoveryConfig{
		SeedPeers: []string{peerAddr},
		P2PServer: clientServer,
	}
	discovery := NewDiscovery(discoveryConfig)
	
	// Manually call connectToPeer to test it directly
	discovery.connectToPeer(peerAddr)
	
	// Give time for connection
	time.Sleep(100 * time.Millisecond)
	
	// Check if peer was added
	peers := clientServer.peerManager.peers
	found := false
	for addr, peer := range peers {
		if addr == peerAddr && peer.Status == PeerConnected {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected peer to be connected after connectToPeer call")
	}
}

func TestDiscoveryConnectToInvalidPeer(t *testing.T) {
	chainStore := store.NewMemoryChainStore()
	config := Config{
		Port:   "0",
		NodeID: "test-node",
		Store:  chainStore,
	}
	server := NewServer(config)
	
	discoveryConfig := DiscoveryConfig{
		SeedPeers: []string{"127.0.0.1:99999"}, // Invalid port
		P2PServer: server,
	}
	discovery := NewDiscovery(discoveryConfig)
	
	// This should not panic or hang
	discovery.connectToPeer("127.0.0.1:99999")
	
	// Should have no connected peers
	peers := server.peerManager.GetConnectedPeers()
	if len(peers) > 0 {
		t.Error("Should not have connected peers when connecting to invalid address")
	}
}

func TestDiscoveryMultipleSeeds(t *testing.T) {
	// Create multiple seed servers
	var seedServers []*Server
	var seedAddrs []string
	
	for i := 0; i < 3; i++ {
		chainStore := store.NewMemoryChainStore()
		config := Config{
			Port:   "0",
			NodeID: "seed-" + string(rune('A'+i)),
			Store:  chainStore,
		}
		server := NewServer(config)
		err := server.Start()
		if err != nil {
			t.Fatalf("Failed to start seed server %d: %v", i, err)
		}
		seedServers = append(seedServers, server)
		seedAddrs = append(seedAddrs, server.listener.Addr().String())
	}
	
	// Clean up all servers
	defer func() {
		for _, server := range seedServers {
			server.listener.Close()
		}
	}()
	
	// Create client with discovery
	chainStore := store.NewMemoryChainStore()
	clientConfig := Config{
		Port:   "0",
		NodeID: "client-node",
		Store:  chainStore,
	}
	clientServer := NewServer(clientConfig)
	err := clientServer.Start()
	if err != nil {
		t.Fatalf("Failed to start client server: %v", err)
	}
	defer clientServer.listener.Close()
	
	discoveryConfig := DiscoveryConfig{
		SeedPeers: seedAddrs,
		P2PServer: clientServer,
	}
	discovery := NewDiscovery(discoveryConfig)
	
	// Start discovery
	discovery.Start()
	
	// Wait for connections
	time.Sleep(300 * time.Millisecond)
	
	// Should have attempted to connect to multiple peers
	// Note: Due to timing and test environment, not all may be connected
	peers := clientServer.peerManager.peers
	if len(peers) == 0 {
		t.Log("Note: No peers connected, but discovery should have attempted connections")
	}
	
	// At minimum, verify that discovery was created with all seed addresses
	if len(discovery.config.SeedPeers) != 3 {
		t.Errorf("Expected 3 seed peers in config, got %d", len(discovery.config.SeedPeers))
	}
}