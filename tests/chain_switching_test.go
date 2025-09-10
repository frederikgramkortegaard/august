package tests

import (
	"crypto/ed25519"
	"crypto/rand"
	"math/big"
	"august/blockchain"
	"august/node"
	"august/p2p"
	"testing"
	"time"
)

func generateTestKey() ed25519.PrivateKey {
	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	return privKey
}

func TestChainSwitching(t *testing.T) {
	// Setup two nodes
	nodeA := node.NewFullNode(node.Config{
		P2PPort:   "19070",
		NodeID:    "node-A",
		SeedPeers: []string{},
	})

	nodeB := node.NewFullNode(node.Config{
		P2PPort:   "19071",
		NodeID:    "node-B",
		SeedPeers: []string{"127.0.0.1:19070"},
	})

	// Start nodes
	<-nodeA.Start()
	<-nodeB.Start()
	defer nodeA.Stop()
	defer nodeB.Stop()

	// Wait for connection
	time.Sleep(100 * time.Millisecond)

	t.Log("=== Nodes started and connected ===")

	// Both nodes should have genesis block
	chainA, err := nodeA.GetP2PServer().GetChainStore().GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain A: %v", err)
	}
	chainB, err := nodeB.GetP2PServer().GetChainStore().GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain B: %v", err)
	}

	if len(chainA.Blocks) != 1 || len(chainB.Blocks) != 1 {
		t.Fatalf("Chains should start with only genesis block")
	}

	// Create test keys
	privKey := generateTestKey()
	pubKey := blockchain.PublicKey{}
	edPubKey := privKey.Public().(ed25519.PublicKey)
	copy(pubKey[:], edPubKey)

	// Mine Block 1 on Node A (lower difficulty)
	block1 := createTestBlock(t, chainA, pubKey, 2, "1") // Difficulty 2

	// Process block 1 on Node A
	<-p2p.ProcessBlock(nodeA.GetP2PServer(), block1)

	// Wait for propagation
	time.Sleep(100 * time.Millisecond)

	// Verify both nodes have block 1
	chainA, _ = nodeA.GetP2PServer().GetChainStore().GetChain()
	chainB, _ = nodeB.GetP2PServer().GetChainStore().GetChain()

	if len(chainA.Blocks) != 2 || len(chainB.Blocks) != 2 {
		t.Fatalf("Both chains should have 2 blocks after block 1")
	}

	t.Log("=== Both nodes have Block 1 ===")

	// Now create a competing fork on Node B with higher difficulty
	// This block also builds on genesis, creating a fork
	chainBGenesis, _ := nodeB.GetP2PServer().GetChainStore().GetChain()
	block2Fork := createTestBlock(t, &blockchain.Chain{
		Blocks:        chainBGenesis.Blocks[:1], // Only genesis
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}, pubKey, 10, "2-fork") // Much higher difficulty!

	t.Logf("Block 1 total work: %s", block1.Header.TotalWork)
	t.Logf("Block 2-fork total work: %s", block2Fork.Header.TotalWork)

	// Process the fork block on Node B
	<-p2p.ProcessBlock(nodeB.GetP2PServer(), block2Fork)

	// Wait for potential reorganization
	time.Sleep(200 * time.Millisecond)

	// Check final state
	chainA, _ = nodeA.GetP2PServer().GetChainStore().GetChain()
	chainB, _ = nodeB.GetP2PServer().GetChainStore().GetChain()

	t.Logf("Final chain A length: %d", len(chainA.Blocks))
	t.Logf("Final chain B length: %d", len(chainB.Blocks))

	// Both should have switched to the higher work chain
	if len(chainA.Blocks) != 2 || len(chainB.Blocks) != 2 {
		t.Fatalf("Both chains should have 2 blocks")
	}

	// Verify both have the high-difficulty block
	finalBlockA := chainA.Blocks[1]
	finalBlockB := chainB.Blocks[1]

	hashA := blockchain.HashBlockHeader(&finalBlockA.Header)
	hashB := blockchain.HashBlockHeader(&finalBlockB.Header)
	hashFork := blockchain.HashBlockHeader(&block2Fork.Header)

	if hashA != hashFork || hashB != hashFork {
		t.Errorf("Both nodes should have switched to the higher work fork")
		t.Logf("Node A has block: %x", hashA[:8])
		t.Logf("Node B has block: %x", hashB[:8])
		t.Logf("Fork block hash: %x", hashFork[:8])
	} else {
		t.Log("✓ Chain reorganization successful - both nodes on higher work chain")
	}

	// Verify total work is correct
	if blockchain.CompareWork(finalBlockA.Header.TotalWork, finalBlockB.Header.TotalWork) != 0 {
		t.Errorf("Both nodes should have same total work")
	}
}

func createTestBlock(t *testing.T, chain *blockchain.Chain, minerPubKey blockchain.PublicKey, difficulty uint64, nonce string) *blockchain.Block {
	latestBlock := chain.Blocks[len(chain.Blocks)-1]
	previousHash := blockchain.HashBlockHeader(&latestBlock.Header)

	// Create coinbase transaction
	coinbase := blockchain.Transaction{
		From:   blockchain.PublicKey{}, // Empty for coinbase
		To:     minerPubKey,
		Amount: blockchain.BlockReward,
		Nonce:  0,
	}

	// Create block parameters with proper work calculation
	// Use unique timestamp to ensure different blocks
	blockParams := blockchain.BlockCreationParams{
		Version:      1,
		PreviousHash: previousHash,
		Height:       uint64(len(chain.Blocks)),
		PreviousWork: latestBlock.Header.TotalWork, // Use actual previous work
		Coinbase:     coinbase,
		Transactions: []blockchain.Transaction{},
		Timestamp:    uint64(time.Now().Unix()) + uint64(len(nonce)), // Unique timestamp before mining
		TargetBits:   blockchain.BigToCompact(big.NewInt(1).Lsh(big.NewInt(1), uint(256-difficulty))),
	}

	// Mine the new block
	newBlock, err := blockchain.NewBlock(blockParams)
	if err != nil {
		t.Fatalf("Failed to create block: %v", err)
	}

	return &newBlock
}
