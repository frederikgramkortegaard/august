package tests

import (
	"encoding/base64"
	"testing"
	"time"
	"gocuria/node"
	"gocuria/blockchain"
)

// TestSingleNodeStartup tests that a single node can start and stop cleanly
func TestSingleNodeStartup(t *testing.T) {
	config := node.Config{
		P2PPort:   "19001", // Use test port range
		NodeID:    "test-node-1",
		SeedPeers: []string{}, // No seed peers for single node test
	}
	
	// Create the node
	testNode := node.NewFullNode(config)
	if testNode == nil {
		t.Fatal("Failed to create test node")
	}
	
	// Start node in goroutine since Start() blocks
	go func() {
		if err := testNode.Start(); err != nil {
			t.Errorf("Failed to start node: %v", err)
		}
	}()
	
	// Give node time to start up
	time.Sleep(100 * time.Millisecond)
	
	// For now, just verify the node was created
	// TODO: Add proper checks for P2P server listening, etc.
	
	// Stop the node
	if err := testNode.Stop(); err != nil {
		t.Errorf("Failed to stop node: %v", err)
	}
}

// TestRequestBlock tests that node A can request the genesis block from node B
func TestRequestBlock(t *testing.T) {
	// Create Node B (has genesis block, acts as seed)
	configB := node.Config{
		P2PPort:   "19002",
		NodeID:    "test-node-B", 
		SeedPeers: []string{},
	}
	
	nodeB := node.NewFullNode(configB)
	if nodeB == nil {
		t.Fatal("Failed to create node B")
	}
	
	// Start node B
	go func() {
		if err := nodeB.Start(); err != nil {
			t.Errorf("Failed to start node B: %v", err)
		}
	}()
	
	// Give node B time to start up
	time.Sleep(150 * time.Millisecond)
	
	// Create Node A (will connect to B and request genesis block)
	configA := node.Config{
		P2PPort:   "19003",
		NodeID:    "test-node-A",
		SeedPeers: []string{"127.0.0.1:19002"}, // Connect to node B
	}
	
	nodeA := node.NewFullNode(configA)
	if nodeA == nil {
		t.Fatal("Failed to create node A")
	}
	
	// Start node A
	go func() {
		if err := nodeA.Start(); err != nil {
			t.Errorf("Failed to start node A: %v", err)
		}
	}()
	
	// Give nodes time to connect
	time.Sleep(200 * time.Millisecond)
	
	// Get the genesis block hash and convert to string
	genesisHash := blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header)
	genesisHashString := base64.StdEncoding.EncodeToString(genesisHash[:])
	
	// Get node A's P2P server to make the request
	p2pServerA := nodeA.GetP2PServer()
	if p2pServerA == nil {
		t.Fatal("Node A P2P server is nil")
	}
	
	// Request the genesis block from node B (now synchronous)
	block, err := p2pServerA.RequestBlockFromPeer("127.0.0.1:19002", genesisHashString)
	if err != nil {
		t.Errorf("Failed to request block from peer: %v", err)
	}
	
	// Verify we received the correct block
	if block != nil {
		receivedHash := blockchain.HashBlockHeader(&block.Header)
		receivedHashString := base64.StdEncoding.EncodeToString(receivedHash[:])
		if receivedHashString != genesisHashString {
			t.Errorf("Received wrong block: expected %s, got %s", genesisHashString, receivedHashString)
		} else {
			t.Logf("Successfully received genesis block from peer")
		}
	} else {
		t.Error("Received nil block")
	}
	
	// Stop both nodes
	if err := nodeA.Stop(); err != nil {
		t.Errorf("Failed to stop node A: %v", err)
	}
	if err := nodeB.Stop(); err != nil {
		t.Errorf("Failed to stop node B: %v", err)
	}
}

// TestPeerSharing tests that nodes can request and share peer lists
func TestPeerSharing(t *testing.T) {
	// Create three nodes: A connects to B, C connects to B
	// Then A should be able to discover C through B's peer sharing
	
	// Node B (seed node)
	nodeB := node.NewFullNode(node.Config{
		P2PPort: "19010", NodeID: "node-B", SeedPeers: []string{},
	})
	go nodeB.Start()
	time.Sleep(100 * time.Millisecond)
	
	// Node C connects to B
	nodeC := node.NewFullNode(node.Config{
		P2PPort: "19011", NodeID: "node-C", SeedPeers: []string{"127.0.0.1:19010"},
	})
	go nodeC.Start()
	time.Sleep(150 * time.Millisecond)
	
	// Node A connects to B
	nodeA := node.NewFullNode(node.Config{
		P2PPort: "19012", NodeID: "node-A", SeedPeers: []string{"127.0.0.1:19010"},
	})
	go nodeA.Start()
	time.Sleep(200 * time.Millisecond) // Allow nodes to connect
	
	// Manually trigger discovery on node A
	<-nodeA.GetDiscovery().RunDiscoveryRound()
	
	// Check that A has discovered C through B's peer sharing
	peerManager := nodeA.GetP2PServer().GetPeerManager()
	connectedPeers := peerManager.GetConnectedPeers()
	
	if len(connectedPeers) < 2 {
		t.Errorf("Expected at least 2 peers (B and C), got %d", len(connectedPeers))
	}
	
	// Cleanup
	nodeA.Stop()
	nodeB.Stop() 
	nodeC.Stop()
}
