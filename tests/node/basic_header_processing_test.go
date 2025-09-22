package main

import (
	"august/node"
	"august/tests"
	"testing"
	"time"
)

func TestBasicHeaderProcessing(t *testing.T) {
	// Create test block
	testBlock := tests.CreateSimpleBlock()

	// Node 1: Peer node with the block data
	node1 := node.NewNode(node.NodeConfig{
		Port:      "8880",
		NodeID:    "Peer-Node",
		SeedPeers: []string{},
		QueryPort: "9990",
	})

	<-node1.Start()

	// Add the block directly to node1's blockchain so it can serve it
	err := node1.AddBlockDirectly(testBlock)
	if err != nil {
		t.Fatalf("Failed to add block to peer node: %v", err)
	}

	// Node 2: Test subject that will receive header and fetch block
	node2 := node.NewNode(node.NodeConfig{
		Port:      "8881",
		NodeID:    "Test-Node",
		SeedPeers: []string{"localhost:8880"}, // Connect to node1
		QueryPort: "9991",
	})

	<-node2.Start()

	// Wait for nodes to connect
	time.Sleep(200 * time.Millisecond)

	// Send only the header to node2 (simulating header-first sync)
	err = node2.ProcessHeader(&testBlock.Header, "peer")
	if err != nil {
		t.Fatalf("Failed to process header: %v", err)
	}

	// Wait for async block fetching and validation
	time.Sleep(500 * time.Millisecond)

	// Verify node2 has successfully processed the block
	currentTip := node2.GetCurrentChain()
	if currentTip == nil {
		t.Fatal("Current chain tip should not be nil after block fetch")
	}

	if currentTip.Header.Height != 1 {
		t.Fatalf("Expected height 1, got %d", currentTip.Header.Height)
	}

	if currentTip.Header.GetHash() != testBlock.Header.GetHash() {
		t.Fatal("Current chain tip should match the processed block")
	}

	// Cleanup
	node1.Stop()
	node2.Stop()
}
