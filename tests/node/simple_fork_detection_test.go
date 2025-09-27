package main

import (
	"august/blockchain"
	"august/config"
	"august/node"
	"august/tests"
	"bytes"
	"encoding/json"
	"github.com/libp2p/go-libp2p/core/peer"
	"net/http"
	"testing"
	"time"
)

func TestSimpleForkDetection(t *testing.T) {
	// Node A gets a longer chain (5 blocks)
	nodeABlocks := tests.CreateBlockChainWithCoinbase(5, config.FirstUser)

	// Node B gets a shorter chain (3 blocks)
	nodeBBlocks := tests.CreateBlockChainWithCoinbase(3, config.FirstUser)

	if nodeBBlocks[1].Header.GetHash() == nodeABlocks[1].Header.GetHash() {
		t.Fatal("yes")
	}

	// Node A: Has the longer/better chain
	nodeA := node.NewNode(node.NodeConfig{
		Port:      8882,
		NodeID:    "Node-A-Long-Chain",
		SeedPeers: []string{},
		QueryPort: 9992,
	})

	<-nodeA.Start()

	// Add longer chain to node A
	for i, block := range nodeABlocks {
		err := nodeA.AddBlockDirectly(block)
		if err != nil {
			t.Fatalf("Failed to add block %d to node A: %v", i+1, err)
		}
	}

	// Get node A's peer info for node B to connect to
	nodeAPeerInfo := nodeA.GetPeerInfo()
	nodeAAddrs, _ := peer.AddrInfoToP2pAddrs(&nodeAPeerInfo)
	t.Logf("Node A multiaddr: %s", nodeAAddrs[0].String())

	// Node B: Has shorter chain, will connect to A and should reorganize
	nodeB := node.NewNode(node.NodeConfig{
		Port:      8883,
		NodeID:    "Node-B-Short-Chain",
		SeedPeers: []string{nodeAAddrs[0].String()}, // Connect to node A
		QueryPort: 9993,
	})

	<-nodeB.Start()

	// Add shorter chain to node B first
	for i, block := range nodeBBlocks {
		err := nodeB.AddBlockDirectly(block)
		if err != nil {
			t.Fatalf("Failed to add block %d to node B: %v", i+1, err)
		}
	}

	// Verify initial state
	chainA := nodeA.GetCurrentChain()
	chainB := nodeB.GetCurrentChain()

	t.Logf("Initial state - Node A height: %d, Node B height: %d", chainA.Header.Height, chainB.Header.Height)

	if chainA.Header.Height != 5 {
		t.Fatalf("Node A should have height 5, got %d", chainA.Header.Height)
	}
	if chainB.Header.Height != 3 {
		t.Fatalf("Node B should have height 3, got %d", chainB.Header.Height)
	}

	// Wait for nodes to connect
	t.Logf("Waiting for nodes to connect...")
	time.Sleep(5000 * time.Millisecond)

	// Add one more block to node A via HTTP API to trigger propagation
	t.Logf("Submitting new block via HTTP API to trigger fork detection...")
	newBlock := tests.CreateTestBlockWithCoinbase(&chainA.Header, config.FirstUser, []*blockchain.Transaction{})

	// Submit block via HTTP API (like a real miner would)
	blockJSON, err := json.Marshal(newBlock)
	if err != nil {
		t.Fatalf("Failed to marshal block: %v", err)
	}

	resp, err := http.Post("http://localhost:9992/submit-block", "application/json", bytes.NewBuffer(blockJSON))
	if err != nil {
		t.Fatalf("Failed to submit block via API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Block submission failed with status: %d", resp.StatusCode)
	}

	t.Logf("Block successfully submitted via API")

	// Wait for block propagation and reorganization
	t.Logf("Waiting for block propagation and reorganization...")
	time.Sleep(30 * time.Second)

	// Check if node B reorganized to match node A's longer chain
	finalChainB := nodeB.GetCurrentChain()
	finalChainA := nodeA.GetCurrentChain()

	if finalChainA == nil {
		t.Fatalf("Node A chain is nil")
	}
	if finalChainB == nil {
		t.Fatalf("Node B chain is nil")
	}

	t.Logf("Final state - Node A height: %d, Node B height: %d", finalChainA.Header.Height, finalChainB.Header.Height)

	if finalChainB.Header.Height != finalChainA.Header.Height {
		t.Fatalf("Fork resolution failed: Node B height %d should match Node A height %d",
			finalChainB.Header.Height, finalChainA.Header.Height)
	}

	if finalChainB.Header.GetHash() != finalChainA.Header.GetHash() {
		t.Fatalf("Fork resolution failed: Node B tip hash should match Node A tip hash")
	}

	// Verify that both nodes have identical chains block by block
	t.Logf("Verifying block-by-block chain identity...")
	for height := uint64(0); height <= finalChainA.Header.Height; height++ {
		blockA := nodeA.GetBlockAtHeight(height)
		blockB := nodeB.GetBlockAtHeight(height)

		if blockA == nil {
			t.Fatalf("Node A missing block at height %d", height)
		}
		if blockB == nil {
			t.Fatalf("Node B missing block at height %d", height)
		}

		hashA := blockA.Header.GetHash()
		hashB := blockB.Header.GetHash()

		if hashA != hashB {
			t.Fatalf("Chain mismatch at height %d: Node A hash=%s, Node B hash=%s",
				height, hashA.String(), hashB.String())
		}

		t.Logf("Height %d: %s", height, hashA.String())
	}

	t.Logf("Fork detection successful! Both nodes have identical chains from height 0 to %d", finalChainA.Header.Height)

	// Print candidate tips map length
	t.Logf("Node A candidate chains: %d", len(nodeA.GetCandidateChains()))
	t.Logf("Node B candidate chains: %d", len(nodeB.GetCandidateChains()))

	// Cleanup
	nodeA.Stop()
	nodeB.Stop()
}
