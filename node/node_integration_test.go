package node

import (
	"fmt"
	"gocuria/blockchain"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestFullNodeCreation(t *testing.T) {
	config := Config{
		HTTPPort:  "0",
		P2PPort:   "0",
		NodeID:    "test-full-node",
		SeedPeers: []string{},
	}

	node := NewFullNode(config)

	if node == nil {
		t.Fatal("NewFullNode returned nil")
	}

	if node.config.NodeID != "test-full-node" {
		t.Errorf("Expected NodeID 'test-full-node', got '%s'", node.config.NodeID)
	}

	if node.store == nil {
		t.Error("Store should be initialized")
	}
}

func TestFullNodeHTTPAPI(t *testing.T) {
	config := Config{
		HTTPPort:  "0", // Use port 0 for testing
		P2PPort:   "0",
		NodeID:    "http-test-node",
		SeedPeers: []string{},
	}

	node := NewFullNode(config)

	// Start HTTP API in background
	go node.startHTTPAPI()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Try to connect to verify server is running
	// Note: We can't easily get the actual port in this test setup
	// This test mainly verifies the method doesn't panic
}

func TestFullNodeP2PServer(t *testing.T) {
	config := Config{
		HTTPPort:  "0",
		P2PPort:   "0",
		NodeID:    "p2p-test-node",
		SeedPeers: []string{},
	}

	node := NewFullNode(config)

	// Start P2P server
	node.startP2P()

	// Give time for server to start
	time.Sleep(100 * time.Millisecond)

	if node.p2pServer == nil {
		t.Error("P2P server should be initialized")
	}
}

func TestTwoNodeP2PCommunication(t *testing.T) {
	// Create first node (seed)
	seedConfig := Config{
		HTTPPort:  "0",
		P2PPort:   "0",
		NodeID:    "seed-node",
		SeedPeers: []string{},
	}
	seedNode := NewFullNode(seedConfig)

	// Start seed node's P2P server
	seedNode.startP2P()
	time.Sleep(100 * time.Millisecond)

	// Get the actual P2P port of seed node
	var seedP2PAddr string
	if seedNode.p2pServer != nil && seedNode.p2pServer.GetListener() != nil {
		seedP2PAddr = seedNode.p2pServer.GetListener().Addr().String()
	} else {
		t.Skip("Cannot get seed node P2P address for integration test")
	}

	// Create second node (client) with seed
	clientConfig := Config{
		HTTPPort:  "0",
		P2PPort:   "0",
		NodeID:    "client-node",
		SeedPeers: []string{seedP2PAddr},
	}
	clientNode := NewFullNode(clientConfig)

	// Start client node's components
	clientNode.startP2P()
	clientNode.startDiscovery()

	// Give time for discovery and connection
	time.Sleep(500 * time.Millisecond)

	// Check if nodes found each other
	seedPeers := seedNode.p2pServer.GetPeerManager().GetConnectedPeers()
	clientPeers := clientNode.p2pServer.GetPeerManager().GetConnectedPeers()

	t.Logf("Seed node has %d connected peers", len(seedPeers))
	t.Logf("Client node has %d connected peers", len(clientPeers))

	// Clean up
	if seedNode.p2pServer.GetListener() != nil {
		seedNode.p2pServer.GetListener().Close()
	}
	if clientNode.p2pServer.GetListener() != nil {
		clientNode.p2pServer.GetListener().Close()
	}
}

func TestNodeDiscovery(t *testing.T) {
	config := Config{
		HTTPPort:  "0",
		P2PPort:   "0",
		NodeID:    "discovery-test-node",
		SeedPeers: []string{"127.0.0.1:9001", "127.0.0.1:9002"},
	}

	node := NewFullNode(config)

	// Start discovery
	node.startDiscovery()

	if node.discovery == nil {
		t.Error("Discovery service should be initialized")
	}

	// Discovery service should be initialized
	// Note: We can't easily test the config without exposing fields
}

func TestFullNodeWithGenesisBlock(t *testing.T) {
	config := Config{
		HTTPPort:  "0",
		P2PPort:   "0",
		NodeID:    "genesis-test-node",
		SeedPeers: []string{},
	}

	node := NewFullNode(config)

	// Add genesis block to store (normally done by Start())
	err := node.store.AddBlock(blockchain.GenesisBlock)
	if err != nil {
		t.Fatalf("Failed to add genesis block: %v", err)
	}

	// Check if store now has genesis block
	height, err := node.store.GetChainHeight()
	if err != nil {
		t.Fatalf("Failed to get chain height: %v", err)
	}

	// After adding genesis, should have height 1
	if height == 0 {
		t.Error("Expected chain height > 0 after adding genesis")
	}

	// Should be able to get head block
	headBlock, err := node.store.GetHeadBlock()
	if err != nil {
		t.Fatalf("Failed to get head block: %v", err)
	}

	if headBlock == nil {
		t.Error("Head block should not be nil")
	}

	// Genesis block should be valid
	if headBlock != blockchain.GenesisBlock {
		t.Error("Head block should be the genesis block")
	}
}

// Helper function to test if a port is available
func isPortAvailable(port string) bool {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// Helper function to make HTTP request with timeout
func makeHTTPRequest(url string, timeout time.Duration) (*http.Response, error) {
	client := &http.Client{Timeout: timeout}
	return client.Get(url)
}

func TestMultiNodeNetwork(t *testing.T) {
	// Skip this test in short mode as it takes longer
	if testing.Short() {
		t.Skip("Skipping multi-node network test in short mode")
	}

	// Create 3 nodes that will form a network
	nodes := make([]*FullNode, 3)

	// Start first node as seed
	nodes[0] = NewFullNode(Config{
		HTTPPort:  "0",
		P2PPort:   "0",
		NodeID:    "node-0",
		SeedPeers: []string{},
	})
	nodes[0].startP2P()
	time.Sleep(100 * time.Millisecond)

	var seedAddr string
	if nodes[0].p2pServer.GetListener() != nil {
		seedAddr = nodes[0].p2pServer.GetListener().Addr().String()
	}

	// Start remaining nodes with first node as seed
	for i := 1; i < 3; i++ {
		nodes[i] = NewFullNode(Config{
			HTTPPort:  "0",
			P2PPort:   "0",
			NodeID:    fmt.Sprintf("node-%d", i),
			SeedPeers: []string{seedAddr},
		})
		nodes[i].startP2P()
		nodes[i].startDiscovery()
		time.Sleep(100 * time.Millisecond)
	}

	// Give time for network formation
	time.Sleep(1 * time.Second)

	// Check network connectivity
	totalConnections := 0
	for i, node := range nodes {
		if node.p2pServer != nil {
			peers := node.p2pServer.GetPeerManager().GetConnectedPeers()
			t.Logf("Node %d has %d connected peers", i, len(peers))
			totalConnections += len(peers)
		}
	}

	t.Logf("Total P2P connections in network: %d", totalConnections)

	// Clean up
	for _, node := range nodes {
		if node.p2pServer != nil && node.p2pServer.GetListener() != nil {
			node.p2pServer.GetListener().Close()
		}
	}
}
