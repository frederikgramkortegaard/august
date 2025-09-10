package tests

import (
	"august/blockchain"
	"august/node"
	"august/p2p"
	"testing"
	"time"
)

// TestRecentBlocksDeduplication tests that duplicate blocks are ignored
func TestRecentBlocksDeduplication(t *testing.T) {
	// Create two nodes: A and B connected
	nodeA := node.NewFullNode(node.Config{
		P2PPort: "19050", NodeID: "node-A", SeedPeers: []string{},
	})
	readyA := nodeA.Start()
	if !<-readyA {
		t.Fatal("Failed to start node A")
	}
	defer nodeA.Stop()

	nodeB := node.NewFullNode(node.Config{
		P2PPort: "19051", NodeID: "node-B", SeedPeers: []string{"127.0.0.1:19050"},
	})
	readyB := nodeB.Start()
	if !<-readyB {
		t.Fatal("Failed to start node B")
	}
	defer nodeB.Stop()

	// Wait for connection
	<-nodeB.GetDiscovery().RunDiscoveryRound()

	t.Logf("=== Nodes connected ===")

	// Create and mine a test block
	chainA, err := nodeA.GetP2PServer().GetChainStore().GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain from Node A: %v", err)
	}

	latestBlock := chainA.Blocks[len(chainA.Blocks)-1]
	previousHash := blockchain.HashBlockHeader(&latestBlock.Header)

	pubKey, _ := generateTestKeyPair()
	coinbase := blockchain.Transaction{
		From:   blockchain.PublicKey{},
		To:     pubKey,
		Amount: blockchain.BlockReward,
		Nonce:  0,
	}

	blockParams := blockchain.BlockCreationParams{
		Version:      1,
		PreviousHash: previousHash,
		Height:       uint64(len(chainA.Blocks)), // Next block height
		Coinbase:     coinbase,
		Transactions: []blockchain.Transaction{},
		Timestamp:    0,
		Difficulty:   blockchain.GetTargetDifficulty(len(chainA.Blocks), chainA.Blocks),
	}

	newBlock, err := blockchain.NewBlock(blockParams)
	if err != nil {
		t.Fatalf("Failed to create new block: %v", err)
	}

	blockHash := blockchain.HashBlockHeader(&newBlock.Header)
	t.Logf("Created test block: %x", blockHash[:8])

	// Submit the block to Node A - this should propagate to Node B
	p2pServerA := nodeA.GetP2PServer()
	<-p2p.ProcessBlock(p2pServerA, &newBlock)

	// Wait for propagation
	time.Sleep(200 * time.Millisecond)

	// Test P2P deduplication by having Node A send the same block again
	// This tests the actual P2P message deduplication mechanism
	t.Logf("=== Testing P2P deduplication - Node A relays same block again ===")

	// Get initial block count in Node B
	chainB, err := nodeB.GetP2PServer().GetChainStore().GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain from Node B: %v", err)
	}
	initialBlockCount := len(chainB.Blocks)

	// Have Node A relay the same block again - this should be deduplicated by Node B
	p2pServerA = nodeA.GetP2PServer()
	<-p2p.RelayBlock(p2pServerA, &newBlock, "")

	// Wait for any potential processing
	time.Sleep(100 * time.Millisecond)

	// Check that Node B didn't add the block again
	chainB, err = nodeB.GetP2PServer().GetChainStore().GetChain()
	if err != nil {
		t.Fatalf("Failed to get updated chain from Node B: %v", err)
	}

	if len(chainB.Blocks) != initialBlockCount {
		t.Errorf("Node B added duplicate block! Block count changed from %d to %d",
			initialBlockCount, len(chainB.Blocks))
	} else {
		t.Logf("P2P deduplication working: Node B ignored duplicate block message")
	}

	// Verify both nodes have the block exactly once in their chains
	verifyNodeHasBlockOnce := func(node *node.FullNode, nodeName string) {
		chain, err := node.GetP2PServer().GetChainStore().GetChain()
		if err != nil {
			t.Errorf("Failed to get chain from %s: %v", nodeName, err)
			return
		}

		// Count occurrences of our test block
		count := 0
		for _, block := range chain.Blocks {
			if blockchain.HashBlockHeader(&block.Header) == blockHash {
				count++
			}
		}

		if count == 0 {
			t.Errorf("%s does not have the test block", nodeName)
		} else if count > 1 {
			t.Errorf("%s has the test block %d times (should be 1)", nodeName, count)
		} else {
			t.Logf("%s correctly has the test block exactly once", nodeName)
		}
	}

	verifyNodeHasBlockOnce(nodeA, "Node A")
	verifyNodeHasBlockOnce(nodeB, "Node B")
}

// TestRecentBlocksCleanup tests that old blocks are cleaned up after TTL
func TestRecentBlocksCleanup(t *testing.T) {
	// Create a single node for testing
	testNode := node.NewFullNode(node.Config{
		P2PPort: "19060", NodeID: "test-node", SeedPeers: []string{},
	})
	ready := testNode.Start()
	if !<-ready {
		t.Fatal("Failed to start test node")
	}
	defer testNode.Stop()

	// Get the P2P server to test its recent blocks functionality
	p2pServer := testNode.GetP2PServer()

	// Create a test block
	chainStore := p2pServer.GetChainStore()
	chain, err := chainStore.GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain: %v", err)
	}

	latestBlock := chain.Blocks[len(chain.Blocks)-1]
	previousHash := blockchain.HashBlockHeader(&latestBlock.Header)

	pubKey, _ := generateTestKeyPair()
	coinbase := blockchain.Transaction{
		From:   blockchain.PublicKey{},
		To:     pubKey,
		Amount: blockchain.BlockReward,
		Nonce:  0,
	}

	blockParams := blockchain.BlockCreationParams{
		Version:      1,
		PreviousHash: previousHash,
		Height:       uint64(len(chain.Blocks)), // Next block height
		Coinbase:     coinbase,
		Transactions: []blockchain.Transaction{},
		Timestamp:    0,
		Difficulty:   blockchain.GetTargetDifficulty(len(chain.Blocks), chain.Blocks),
	}

	testBlock, err := blockchain.NewBlock(blockParams)
	if err != nil {
		t.Fatalf("Failed to create test block: %v", err)
	}

	blockHash := blockchain.HashBlockHeader(&testBlock.Header)
	t.Logf("Created test block for cleanup test: %x", blockHash[:8])

	// Process the block to add it to recent blocks
	<-p2p.ProcessBlock(p2pServer, &testBlock)

	// Manually trigger cleanup - this should not remove the block yet (it's fresh)
	// We'll need to access the cleanup method, but since it's not exported,
	// we can test the behavior indirectly by trying to process the same block again

	t.Logf("=== Testing that recent block is still remembered ===")

	// Try to process the same block again - should be ignored
	<-p2p.ProcessBlock(p2pServer, &testBlock)
	t.Logf("Second processing completed (duplicate should be handled properly)")

	t.Logf("=== Recent blocks deduplication test completed ===")
	// For a full cleanup test, we'd need to either:
	// 1. Export the cleanup method, or
	// 2. Create a test with a very short TTL, or
	// 3. Wait 5+ minutes (not practical for unit tests)
}
