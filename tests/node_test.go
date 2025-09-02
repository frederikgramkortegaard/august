package tests

import (
	"encoding/base64"
	"august/blockchain"
	"august/node"
	"august/p2p"
	"testing"
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

	// Start node and wait for it to be ready
	ready := testNode.Start()
	if !<-ready {
		t.Fatal("Failed to start test node")
	}

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
	readyB := nodeB.Start()
	if !<-readyB {
		t.Fatal("Failed to start node B")
	}

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
	readyA := nodeA.Start()
	if !<-readyA {
		t.Fatal("Failed to start node A")
	}

	// Wait for nodes to connect - RunDiscoveryRound now waits for actual connections
	<-nodeA.GetDiscovery().RunDiscoveryRound()

	// Get the genesis block hash and convert to string
	genesisHash := blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header)
	genesisHashString := base64.StdEncoding.EncodeToString(genesisHash[:])

	// Get node A's P2P server to make the request
	p2pServerA := nodeA.GetP2PServer()
	if p2pServerA == nil {
		t.Fatal("Node A P2P server is nil")
	}

	// Request the genesis block from node B (now synchronous)
	block, err := p2p.RequestBlockFromPeer(p2pServerA, "127.0.0.1:19002", genesisHashString)
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
	readyB := nodeB.Start()
	if !<-readyB {
		t.Fatal("Failed to start node B")
	}

	// Node C connects to B
	nodeC := node.NewFullNode(node.Config{
		P2PPort: "19011", NodeID: "node-C", SeedPeers: []string{"127.0.0.1:19010"},
	})
	readyC := nodeC.Start()
	if !<-readyC {
		t.Fatal("Failed to start node C")
	}

	// Node A connects to B
	nodeA := node.NewFullNode(node.Config{
		P2PPort: "19012", NodeID: "node-A", SeedPeers: []string{"127.0.0.1:19010"},
	})
	readyA := nodeA.Start()
	if !<-readyA {
		t.Fatal("Failed to start node A")
	}

	// Manually trigger discovery on node A
	<-nodeA.GetDiscovery().RunDiscoveryRound()
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
