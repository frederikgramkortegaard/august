package tests

import (
	"crypto/ed25519"
	"crypto/rand"
	"gocuria/blockchain"
	"gocuria/node"
	"gocuria/p2p"
	"testing"
	"time"
)

func TestHeaderFirstDiscovery(t *testing.T) {
	t.Log("=== Testing Headers-First Discovery Protocol ===")

	// Create test nodes
	nodeA := node.NewFullNode(node.Config{
		P2PPort:   "19080",
		NodeID:    "node-A",
		SeedPeers: []string{},
	})

	nodeB := node.NewFullNode(node.Config{
		P2PPort:   "19081",
		NodeID:    "node-B",
		SeedPeers: []string{"127.0.0.1:19080"},
	})

	// Start nodes
	<-nodeA.Start()
	<-nodeB.Start()
	defer nodeA.Stop()
	defer nodeB.Stop()

	// Wait for connection
	time.Sleep(200 * time.Millisecond)
	t.Log("Nodes connected")

	// Create test key for mining
	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	pubKey := blockchain.PublicKey{}
	edPubKey := privKey.Public().(ed25519.PublicKey)
	copy(pubKey[:], edPubKey)

	// Phase 1: Create a longer chain on Node A (but don't broadcast blocks yet)
	t.Log("=== Phase 1: Node A builds longer chain privately ===")

	chainA, _ := nodeA.GetP2PServer().GetChainStore().GetChain()

	// Mine 5 blocks privately on Node A
	var blocksA []*blockchain.Block
	currentChain := chainA
	for i := 1; i <= 5; i++ {
		block := createTestBlockWithDifficulty(t, currentChain, pubKey, 5, i)
		blocksA = append(blocksA, block)

		// Add to Node A's chain directly (simulate private mining)
		err := nodeA.GetP2PServer().GetChainStore().AddBlock(block)
		if err != nil {
			t.Fatalf("Failed to add block %d to Node A: %v", i, err)
		}

		// Update current chain for next block
		currentChain, _ = nodeA.GetP2PServer().GetChainStore().GetChain()
		t.Logf("Node A mined block %d at height %d", i, block.Header.Height)
	}

	finalChainA, _ := nodeA.GetP2PServer().GetChainStore().GetChain()
	t.Logf("Node A now has %d blocks", len(finalChainA.Blocks))

	// Verify Node B is still at genesis
	chainB, _ := nodeB.GetP2PServer().GetChainStore().GetChain()
	if len(chainB.Blocks) != 1 {
		t.Fatalf("Node B should still be at genesis, has %d blocks", len(chainB.Blocks))
	}

	// Phase 2: Trigger headers-first discovery
	t.Log("=== Phase 2: Testing Headers-First Protocol ===")

	// Create chain head payload representing Node A's better chain
	headBlock := finalChainA.Blocks[len(finalChainA.Blocks)-1]
	headHash := blockchain.HashBlockHeader(&headBlock.Header)

	chainHead := &p2p.ChainHeadPayload{
		Height:    headBlock.Header.Height,
		HeadHash:  blockchain.EncodeHash(headHash),
		TotalWork: headBlock.Header.TotalWork,
		Header:    headBlock.Header,
	}

	// Node B evaluates Node A's chain (this should trigger headers-first download)
	t.Log("Node B starting headers-first evaluation of Node A's chain")
	serverB := nodeB.GetP2PServer()

	// This should:
	// 1. Download headers first
	// 2. Validate header chain
	// 3. Start candidate chain download
	err := p2p.EvaluateChainHead(serverB, "127.0.0.1:19080", chainHead)
	if err != nil {
		t.Fatalf("Headers-first evaluation failed: %v", err)
	}

	// Phase 3: Verify headers-first behavior
	t.Log("=== Phase 3: Verifying Headers-First Behavior ===")

	// Wait for headers download and candidate creation
	time.Sleep(300 * time.Millisecond)

	// Check that Node B created a candidate chain
	candidateFound := false
	serverB.GetCandidateChains().Range(func(key, value interface{}) bool {
		candidate := value.(*p2p.CandidateChain)
		t.Logf("Found candidate chain: %s from %s", candidate.ID, candidate.PeerSource)

		// Verify the candidate has headers
		if len(candidate.Headers) != 5 {
			t.Errorf("Expected 5 headers in candidate, got %d", len(candidate.Headers))
		}

		// Verify expected work matches
		if candidate.ExpectedWork != chainHead.TotalWork {
			t.Errorf("Expected work mismatch: got %s, want %s", candidate.ExpectedWork, chainHead.TotalWork)
		}

		candidateFound = true
		return false // stop iteration
	})

	if !candidateFound {
		t.Fatal("No candidate chain found - headers-first discovery failed")
	}

	// Phase 4: Wait for full synchronization
	t.Log("=== Phase 4: Waiting for Full Block Download ===")

	// Wait for block download to complete
	maxWait := 30 * time.Second
	startWait := time.Now()

	for time.Since(startWait) < maxWait {
		chainB, _ = nodeB.GetP2PServer().GetChainStore().GetChain()
		if len(chainB.Blocks) == len(finalChainA.Blocks) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Phase 5: Verify final synchronization
	t.Log("=== Phase 5: Verifying Final Synchronization ===")

	chainB, _ = nodeB.GetP2PServer().GetChainStore().GetChain()
	if len(chainB.Blocks) != len(finalChainA.Blocks) {
		t.Fatalf("Synchronization incomplete: Node B has %d blocks, Node A has %d",
			len(chainB.Blocks), len(finalChainA.Blocks))
	}

	// Verify both chains have same head hash
	headA := blockchain.HashBlockHeader(&finalChainA.Blocks[len(finalChainA.Blocks)-1].Header)
	headB := blockchain.HashBlockHeader(&chainB.Blocks[len(chainB.Blocks)-1].Header)

	if headA != headB {
		t.Fatalf("Chain heads don't match after sync: A=%x, B=%x", headA[:8], headB[:8])
	}

	// Verify both chains have same total work
	workA := finalChainA.Blocks[len(finalChainA.Blocks)-1].Header.TotalWork
	workB := chainB.Blocks[len(chainB.Blocks)-1].Header.TotalWork

	if workA != workB {
		t.Fatalf("Total work doesn't match: A=%s, B=%s", workA, workB)
	}

	// Phase 6: Verify cleanup
	t.Log("=== Phase 6: Verifying Candidate Cleanup ===")

	// After successful sync, candidate chains should be cleaned up
	candidateCount := 0
	serverB.GetCandidateChains().Range(func(key, value interface{}) bool {
		candidateCount++
		return true
	})

	if candidateCount != 0 {
		t.Errorf("Expected 0 candidate chains after sync, found %d", candidateCount)
	}

	t.Log("Headers-first discovery and synchronization successful!")
}

func TestHeaderFirstRejectsBadChains(t *testing.T) {
	t.Log("=== Testing Headers-First Rejects Invalid Chains ===")

	// Create test node
	node := node.NewFullNode(node.Config{
		P2PPort:   "19082",
		NodeID:    "test-node",
		SeedPeers: []string{},
	})

	<-node.Start()
	defer node.Stop()

	// Create fake chain head with invalid data
	fakeChainHead := &p2p.ChainHeadPayload{
		Height:    100,
		HeadHash:  "fake-hash-data-invalid",
		TotalWork: "999999999", // Impossibly high work
		Header: blockchain.BlockHeader{
			Height: 100,
			// Invalid header data
		},
	}

	server := node.GetP2PServer()

	// This should fail during header validation
	_ = p2p.EvaluateChainHead(server, "127.0.0.1:19999", fakeChainHead)

	// The function might return an error or handle it internally
	// Check that no candidate chains were created
	time.Sleep(100 * time.Millisecond)

	candidateCount := 0
	server.GetCandidateChains().Range(func(key, value interface{}) bool {
		candidateCount++
		return true
	})

	if candidateCount > 0 {
		t.Errorf("Expected no candidate chains for invalid chain, found %d", candidateCount)
	}

	t.Log("Headers-first correctly rejected invalid chain")
}

func createTestBlockWithDifficulty(t *testing.T, chain *blockchain.Chain, minerPubKey blockchain.PublicKey, difficulty uint64, nonce int) *blockchain.Block {
	latestBlock := chain.Blocks[len(chain.Blocks)-1]
	previousHash := blockchain.HashBlockHeader(&latestBlock.Header)

	// Create coinbase transaction
	coinbase := blockchain.Transaction{
		From:   blockchain.PublicKey{}, // Empty for coinbase
		To:     minerPubKey,
		Amount: 50,
		Nonce:  0,
	}

	// Create block with unique timestamp
	blockParams := blockchain.BlockCreationParams{
		Version:      1,
		PreviousHash: previousHash,
		Height:       uint64(len(chain.Blocks)),
		PreviousWork: latestBlock.Header.TotalWork,
		Coinbase:     coinbase,
		Transactions: []blockchain.Transaction{},
		Timestamp:    uint64(time.Now().Unix()) + uint64(nonce*1000), // Unique timestamp
		Difficulty:   difficulty,
	}

	// Mine the block
	newBlock, err := blockchain.NewBlock(blockParams)
	if err != nil {
		t.Fatalf("Failed to create block: %v", err)
	}

	return &newBlock
}
