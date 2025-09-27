package main

import (
	"august/config"
	"august/node"
	"august/tests"
	"testing"
	"time"
)

func TestBasicHeaderProcessing(t *testing.T) {
	_NUM_BLOCKS_TO_ADD := 3

	// Create test blocks chain
	testBlocks := tests.CreateBlockChainWithCoinbase(_NUM_BLOCKS_TO_ADD, config.FirstUser)

	// Node 1: Peer node with the block data
	node1 := node.NewNode(node.NodeConfig{
		Port:      "8880",
		NodeID:    "Peer-Node",
		SeedPeers: []string{},
		QueryPort: 9990,
	})

	<-node1.Start()

	// Add all blocks directly to node1's blockchain so it can serve them
	for i, block := range testBlocks {
		err := node1.AddBlockDirectly(block)
		if err != nil {
			t.Fatalf("Failed to add block %d to peer node: %v", i+1, err)
		}
	}

	// Node 2: Test subject that will receive header and fetch block
	node2 := node.NewNode(node.NodeConfig{
		Port:      "8881",
		NodeID:    "Test-Node",
		SeedPeers: []string{"localhost:8880"}, // Connect to node1
		QueryPort: 9991,
	})

	<-node2.Start()

	// Wait for nodes to connect
	time.Sleep(200 * time.Millisecond)

	// Process blocks sequentially (simulating header-first sync)
	for i, block := range testBlocks {
		t.Logf("Processing block %d (height %d)", i+1, block.Header.Height)

		// Send only the header to node2 (simulating header-first sync)
		err := node2.ProcessHeader(&block.Header)
		if err != nil {
			t.Fatalf("Failed to process header %d: %v", i+1, err)
		}

		// Wait for async block fetching and validation
		time.Sleep(300 * time.Millisecond)

		// Verify node2 has successfully processed the block
		currentTip := node2.GetCurrentChain()
		if currentTip == nil {
			t.Fatalf("Current chain tip should not be nil after block %d fetch", i+1)
		}

		expectedHeight := uint64(i + 1)
		if currentTip.Header.Height != expectedHeight {
			t.Fatalf("Block %d: Expected height %d, got %d", i+1, expectedHeight, currentTip.Header.Height)
		}

		if currentTip.Header.GetHash() != block.Header.GetHash() {
			t.Fatalf("Block %d: Current chain tip should match the processed block", i+1)
		}

		t.Logf("Block %d successfully processed and validated", i+1)
	}

	t.Logf("All %d blocks processed successfully!", _NUM_BLOCKS_TO_ADD)

	// Cleanup
	node1.Stop()
	node2.Stop()
}
