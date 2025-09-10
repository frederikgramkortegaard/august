package tests

import (
	"crypto/ed25519"
	"crypto/rand"
	"august/blockchain"
	"august/node"
	"august/p2p"
	"testing"
	"time"
)

// Test helper functions

// generateTestKeyPair creates a new ed25519 key pair for testing
func generateTestKeyPair() (blockchain.PublicKey, ed25519.PrivateKey) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)

	var pubKey blockchain.PublicKey
	copy(pubKey[:], publicKey)

	return pubKey, privateKey
}

// TestBlockPropagation tests that new blocks propagate between connected nodes
func TestBlockPropagation(t *testing.T) {
	// Create 3 nodes: A -> B -> C (linear chain)

	// Node A (genesis holder, will mine a new block)
	nodeA := node.NewFullNode(node.Config{
		P2PPort: "19030", NodeID: "node-A", SeedPeers: []string{},
	})
	readyA := nodeA.Start()
	if !<-readyA {
		t.Fatal("Failed to start node A")
	}

	// Node B (connects to A)
	nodeB := node.NewFullNode(node.Config{
		P2PPort: "19031", NodeID: "node-B", SeedPeers: []string{"127.0.0.1:19030"},
	})
	readyB := nodeB.Start()
	if !<-readyB {
		t.Fatal("Failed to start node B")
	}

	// Node C (connects to B)
	nodeC := node.NewFullNode(node.Config{
		P2PPort: "19032", NodeID: "node-C", SeedPeers: []string{"127.0.0.1:19031"},
	})
	readyC := nodeC.Start()
	if !<-readyC {
		t.Fatal("Failed to start node C")
	}

	// Wait for peer connections to establish
	<-nodeB.GetDiscovery().RunDiscoveryRound() // B connects to A
	<-nodeC.GetDiscovery().RunDiscoveryRound() // C connects to B

	t.Logf("=== All nodes connected ===")

	// Create and mine a new block on Node A
	// Get the current chain state from Node A
	chainA, err := nodeA.GetP2PServer().GetChainStore().GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain from Node A: %v", err)
	}

	// Get the latest block hash for previous hash
	latestBlock := chainA.Blocks[len(chainA.Blocks)-1]
	previousHash := blockchain.HashBlockHeader(&latestBlock.Header)

	// Create a simple coinbase transaction (miner reward)
	pubKey, _ := generateTestKeyPair()
	coinbase := blockchain.Transaction{
		From:   blockchain.PublicKey{}, // Empty for coinbase
		To:     pubKey,
		Amount: 50,
		Nonce:  0,
	}

	// Get the latest block to get previous work
	latestBlock = chainA.Blocks[len(chainA.Blocks)-1]
	
	// Get target bits for this height
	targetBits := blockchain.GetTargetBits(len(chainA.Blocks), chainA.Blocks)
	
	// Create new block parameters
	blockParams := blockchain.BlockCreationParams{
		Version:      1,
		PreviousHash: previousHash,
		Height:       uint64(len(chainA.Blocks)), // Next block height
		PreviousWork: latestBlock.Header.TotalWork,
		Coinbase:     coinbase,
		Transactions: []blockchain.Transaction{}, // No additional transactions
		Timestamp:    0,                          // Will use current time
		TargetBits:   targetBits,
	}

	// Mine the new block
	difficulty := blockchain.CalculateDifficulty(targetBits)
	diffFloat, _ := difficulty.Float64()
	t.Logf("Mining new block with difficulty %.2f...", diffFloat)
	newBlock, err := blockchain.NewBlock(blockParams)
	if err != nil {
		t.Fatalf("Failed to create new block: %v", err)
	}

	blockHash := blockchain.HashBlockHeader(&newBlock.Header)
	t.Logf("Mined new block: %x", blockHash[:8])

	// Submit the block to Node A - this should trigger propagation A -> B -> C
	p2pServerA := nodeA.GetP2PServer()
	<-p2p.ProcessBlock(p2pServerA, &newBlock)

	// Wait a bit for propagation
	time.Sleep(100 * time.Millisecond)

	// Verify all nodes received the block
	verifyNodeHasBlock := func(node *node.FullNode, nodeName string) {
		chain, err := node.GetP2PServer().GetChainStore().GetChain()
		if err != nil {
			t.Errorf("Failed to get chain from %s: %v", nodeName, err)
			return
		}

		// Check if the new block is in the chain
		found := false
		for _, block := range chain.Blocks {
			if blockchain.HashBlockHeader(&block.Header) == blockHash {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("%s did not receive the new block %x", nodeName, blockHash[:8])
		} else {
			t.Logf("%s successfully received block %x", nodeName, blockHash[:8])
		}
	}

	// Verify all nodes have the block
	verifyNodeHasBlock(nodeA, "Node A")
	verifyNodeHasBlock(nodeB, "Node B")
	verifyNodeHasBlock(nodeC, "Node C")

	// Cleanup
	nodeA.Stop()
	nodeB.Stop()
	nodeC.Stop()
}

// TestBlockRelay tests that blocks are correctly relayed to all peers except sender
func TestBlockRelay(t *testing.T) {
	// Create 4 nodes in a more complex topology:
	//     A
	//   / | \
	//  B  C  D
	// Node A is connected to B, C, and D

	// Node A (center node)
	nodeA := node.NewFullNode(node.Config{
		P2PPort: "19040", NodeID: "node-A", SeedPeers: []string{},
	})
	readyA := nodeA.Start()
	if !<-readyA {
		t.Fatal("Failed to start node A")
	}

	// Node B (connects to A)
	nodeB := node.NewFullNode(node.Config{
		P2PPort: "19041", NodeID: "node-B", SeedPeers: []string{"127.0.0.1:19040"},
	})
	readyB := nodeB.Start()
	if !<-readyB {
		t.Fatal("Failed to start node B")
	}

	// Node C (connects to A)
	nodeC := node.NewFullNode(node.Config{
		P2PPort: "19042", NodeID: "node-C", SeedPeers: []string{"127.0.0.1:19040"},
	})
	readyC := nodeC.Start()
	if !<-readyC {
		t.Fatal("Failed to start node C")
	}

	// Node D (connects to A)
	nodeD := node.NewFullNode(node.Config{
		P2PPort: "19043", NodeID: "node-D", SeedPeers: []string{"127.0.0.1:19040"},
	})
	readyD := nodeD.Start()
	if !<-readyD {
		t.Fatal("Failed to start node D")
	}

	// Wait for all nodes to connect to A
	<-nodeB.GetDiscovery().RunDiscoveryRound() // B connects to A
	<-nodeC.GetDiscovery().RunDiscoveryRound() // C connects to A
	<-nodeD.GetDiscovery().RunDiscoveryRound() // D connects to A

	t.Logf("=== All nodes connected to A ===")

	// TODO: Node B creates and broadcasts a new block
	// The block should: B -> A -> C, D (but not back to B)

	// TODO: Verify relay logic works correctly

	// Cleanup
	nodeA.Stop()
	nodeB.Stop()
	nodeC.Stop()
	nodeD.Stop()
}
