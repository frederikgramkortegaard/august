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

func TestLongestChainResolution(t *testing.T) {
	t.Log("=== Testing Longest Chain Resolution with Multiple Candidates ===")

	// Create test key for mining
	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	pubKey := blockchain.PublicKey{}
	edPubKey := privKey.Public().(ed25519.PublicKey)
	copy(pubKey[:], edPubKey)

	// Create Node A that will have Chain A (3 blocks, low difficulty)
	nodeA := node.NewFullNode(node.Config{
		P2PPort: "19090", NodeID: "node-A", SeedPeers: []string{},
	})
	if !<-nodeA.Start() {
		t.Fatal("Failed to start node A")
	}
	defer nodeA.Stop()

	// Create Node B that will have Chain B (4 blocks, medium difficulty)
	nodeB := node.NewFullNode(node.Config{
		P2PPort: "19091", NodeID: "node-B", SeedPeers: []string{},
	})
	if !<-nodeB.Start() {
		t.Fatal("Failed to start node B")
	}
	defer nodeB.Stop()

	// Create Node C that will have Chain C (2 blocks, very high difficulty - should win!)
	nodeC := node.NewFullNode(node.Config{
		P2PPort: "19092", NodeID: "node-C", SeedPeers: []string{},
	})
	if !<-nodeC.Start() {
		t.Fatal("Failed to start node C")
	}
	defer nodeC.Stop()

	// Create evaluator node that will receive all chains and choose the best
	evaluator := node.NewFullNode(node.Config{
		P2PPort: "19093", NodeID: "evaluator",
		SeedPeers: []string{"127.0.0.1:19090", "127.0.0.1:19091", "127.0.0.1:19092"},
	})
	if !<-evaluator.Start() {
		t.Fatal("Failed to start evaluator node")
	}
	defer evaluator.Stop()

	t.Log("=== Phase 1: Building Different Chains on Each Node ===")

	// Build Chain A on Node A: 3 blocks with difficulty 2 each (total work = 7)
	serverA := nodeA.GetP2PServer()
	chainA, _ := serverA.GetChainStore().GetChain()
	for i := 0; i < 3; i++ {
		block := createTestBlockWithDifficulty(t, chainA, pubKey, 2, i+1)
		err := serverA.GetChainStore().AddBlock(block)
		if err != nil {
			t.Fatalf("Failed to add block %d to Chain A: %v", i+1, err)
		}
		chainA, _ = serverA.GetChainStore().GetChain()
	}

	// Build Chain B on Node B: 4 blocks with difficulty 3 each (total work = 13)
	serverB := nodeB.GetP2PServer()
	chainB, _ := serverB.GetChainStore().GetChain()
	for i := 0; i < 4; i++ {
		block := createTestBlockWithDifficulty(t, chainB, pubKey, 3, i+10)
		err := serverB.GetChainStore().AddBlock(block)
		if err != nil {
			t.Fatalf("Failed to add block %d to Chain B: %v", i+1, err)
		}
		chainB, _ = serverB.GetChainStore().GetChain()
	}

	// Build Chain C on Node C: 2 blocks with difficulty 20 each (total work = 41 - should win!)
	serverC := nodeC.GetP2PServer()
	chainC, _ := serverC.GetChainStore().GetChain()
	for i := 0; i < 2; i++ {
		block := createTestBlockWithDifficulty(t, chainC, pubKey, 20, i+20)
		err := serverC.GetChainStore().AddBlock(block)
		if err != nil {
			t.Fatalf("Failed to add block %d to Chain C: %v", i+1, err)
		}
		chainC, _ = serverC.GetChainStore().GetChain()
	}

	// Log chain info
	finalChainA, _ := serverA.GetChainStore().GetChain()
	finalChainB, _ := serverB.GetChainStore().GetChain()
	finalChainC, _ := serverC.GetChainStore().GetChain()

	workA := finalChainA.Blocks[len(finalChainA.Blocks)-1].Header.TotalWork
	workB := finalChainB.Blocks[len(finalChainB.Blocks)-1].Header.TotalWork
	workC := finalChainC.Blocks[len(finalChainC.Blocks)-1].Header.TotalWork

	t.Logf("Chain A: %d blocks, work=%s", len(finalChainA.Blocks)-1, workA)
	t.Logf("Chain B: %d blocks, work=%s", len(finalChainB.Blocks)-1, workB)
	t.Logf("Chain C: %d blocks, work=%s", len(finalChainC.Blocks)-1, workC)

	// Verify Chain C has the most work (should be chosen)
	if blockchain.CompareWork(workC, workA) <= 0 {
		t.Fatal("Chain C should have more work than Chain A")
	}
	if blockchain.CompareWork(workC, workB) <= 0 {
		t.Fatal("Chain C should have more work than Chain B")
	}

	t.Log("=== Phase 2: Connecting Evaluator to Other Nodes ===")

	// Connect evaluator to all other nodes
	<-evaluator.GetDiscovery().RunDiscoveryRound()
	time.Sleep(500 * time.Millisecond) // Let connections establish

	// Get evaluator server for chain operations
	evaluatorServer := evaluator.GetP2PServer()

	t.Log("=== Phase 3: Sending Chain Head Announcements ===")

	// Create chain head payloads to announce better chains
	headA := createChainHeadPayload(finalChainA, "127.0.0.1:19090")
	headB := createChainHeadPayload(finalChainB, "127.0.0.1:19091")
	headC := createChainHeadPayload(finalChainC, "127.0.0.1:19092")

	// Send chain heads to evaluator in worst-to-best order
	t.Log("Sending Chain A head to evaluator")
	err := p2p.EvaluateChainHead(evaluatorServer, "127.0.0.1:19090", headA)
	if err != nil {
		t.Logf("Chain A evaluation: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	t.Log("Sending Chain B head to evaluator")
	err = p2p.EvaluateChainHead(evaluatorServer, "127.0.0.1:19091", headB)
	if err != nil {
		t.Logf("Chain B evaluation: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	t.Log("Sending Chain C head to evaluator (should win!)")
	err = p2p.EvaluateChainHead(evaluatorServer, "127.0.0.1:19092", headC)
	if err != nil {
		t.Logf("Chain C evaluation: %v", err)
	}

	t.Log("=== Phase 4: Monitoring Chain Resolution ===")

	// Wait for chain synchronization and candidate evaluation
	maxWait := 30 * time.Second
	startWait := time.Now()
	resolved := false

	for time.Since(startWait) < maxWait {
		currentChain, _ := evaluatorServer.GetChainStore().GetChain()

		// Check if evaluator has adopted the best chain (Chain C)
		if len(currentChain.Blocks) > 1 {
			finalWork := currentChain.Blocks[len(currentChain.Blocks)-1].Header.TotalWork
			if finalWork == workC {
				t.Logf("Evaluator adopted Chain C with work %s", finalWork)
				resolved = true
				break
			}
		}

		// Log candidates periodically
		candidateCount := 0
		evaluatorServer.GetCandidateChains().Range(func(key, value interface{}) bool {
			candidateCount++
			return true
		})
		if candidateCount > 0 {
			t.Logf("Active candidates: %d", candidateCount)
		}

		time.Sleep(500 * time.Millisecond)
	}

	if !resolved {
		t.Fatal("Chain resolution did not complete within timeout")
	}

	t.Log("=== Phase 5: Verifying Optimal Chain Selection ===")

	finalChain, _ := evaluatorServer.GetChainStore().GetChain()

	// Debug information
	t.Logf("Chain C original length: %d blocks", len(finalChainC.Blocks))
	t.Logf("Evaluator final length: %d blocks", len(finalChain.Blocks))
	t.Logf("Chain C work: %s", workC)
	t.Logf("Evaluator final work: %s", finalChain.Blocks[len(finalChain.Blocks)-1].Header.TotalWork)

	// The key test: does the evaluator have Chain C's work? (That's what matters for longest chain)
	finalWork := finalChain.Blocks[len(finalChain.Blocks)-1].Header.TotalWork
	if finalWork != workC {
		t.Fatalf("Expected Chain C work %s, got %s", workC, finalWork)
	}

	t.Logf("Correct work achieved - longest chain rule working!")

	// Verify block content matches Chain C (compare the head block hash)
	finalHead := blockchain.HashBlockHeader(&finalChain.Blocks[len(finalChain.Blocks)-1].Header)
	chainCExpectedHead := blockchain.HashBlockHeader(&finalChainC.Blocks[len(finalChainC.Blocks)-1].Header)

	if finalHead != chainCExpectedHead {
		t.Fatalf("Final chain head doesn't match Chain C: got %x, expected %x",
			finalHead[:8], chainCExpectedHead[:8])
	}

	t.Log("=== Phase 6: Verifying Cleanup of Losing Candidates ===")

	// After resolution, losing candidates should be cleaned up
	time.Sleep(200 * time.Millisecond)

	remainingCandidates := 0
	evaluatorServer.GetCandidateChains().Range(func(key, value interface{}) bool {
		remainingCandidates++
		return true
	})

	if remainingCandidates != 0 {
		t.Errorf("Expected 0 remaining candidates after resolution, found %d", remainingCandidates)
	}

	t.Log("Longest chain resolution successful - Chain C (highest work) was chosen!")
}

func TestConcurrentChainEvaluation(t *testing.T) {
	t.Log("=== Testing Concurrent Chain Evaluation (No Blocking) ===")

	evaluatorNode := node.NewFullNode(node.Config{
		P2PPort:   "19091",
		NodeID:    "concurrent-evaluator",
		SeedPeers: []string{},
	})

	<-evaluatorNode.Start()
	defer evaluatorNode.Stop()

	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	pubKey := blockchain.PublicKey{}
	edPubKey := privKey.Public().(ed25519.PublicKey)
	copy(pubKey[:], edPubKey)

	server := evaluatorNode.GetP2PServer()
	startChain, _ := server.GetChainStore().GetChain()

	t.Log("Creating multiple chains for concurrent evaluation")

	// Create 5 different chains with varying characteristics
	chains := []*blockchain.Chain{}
	chainHeads := []*p2p.ChainHeadPayload{}

	chainConfigs := []struct {
		name         string
		difficulties []uint64
		peer         string
	}{
		{"fast-chain", []uint64{1, 1, 1, 1, 1, 1}, "peer-fast"},
		{"medium-chain", []uint64{3, 3, 3, 3}, "peer-medium"},
		{"slow-chain", []uint64{10, 10}, "peer-slow"},
		{"uneven-chain", []uint64{1, 5, 1, 5}, "peer-uneven"},
		{"winner-chain", []uint64{15, 15}, "peer-winner"}, // Should win
	}

	for _, config := range chainConfigs {
		chain := buildTestChain(t, startChain, pubKey, config.difficulties, config.name)
		chainHead := createChainHeadPayload(chain, config.peer)
		chains = append(chains, chain)
		chainHeads = append(chainHeads, chainHead)
		t.Logf("%s: %d blocks, work=%s", config.name, len(chain.Blocks)-1, chainHead.TotalWork)
	}

	t.Log("Starting concurrent evaluations (should not block each other)")

	// Start all evaluations simultaneously
	startTime := time.Now()
	for i, chainHead := range chainHeads {
		go func(idx int, head *p2p.ChainHeadPayload, peer string) {
			err := p2p.EvaluateChainHead(server, peer, head)
			if err != nil {
				t.Logf("Chain %d evaluation failed: %v", idx, err)
			}
		}(i, chainHead, chainConfigs[i].peer)
	}

	// All evaluations should start quickly (no blocking)
	evalStartTime := time.Since(startTime)
	if evalStartTime > 500*time.Millisecond {
		t.Errorf("Concurrent evaluations took too long to start: %v", evalStartTime)
	}

	t.Log("Waiting for resolution with multiple concurrent downloads")

	// Wait for resolution
	maxWait := 45 * time.Second
	startWait := time.Now()

	for time.Since(startWait) < maxWait {
		currentChain, _ := server.GetChainStore().GetChain()
		if len(currentChain.Blocks) > 1 {
			finalWork := currentChain.Blocks[len(currentChain.Blocks)-1].Header.TotalWork
			// Check if we got the winner chain (should have highest work)
			if finalWork == chainHeads[4].TotalWork { // winner-chain
				t.Log("Concurrent evaluation resolved to optimal chain")
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatal("Concurrent chain evaluation did not resolve within timeout")
}

// Helper functions

func buildTestChain(t *testing.T, baseChain *blockchain.Chain, minerPubKey blockchain.PublicKey, difficulties []uint64, chainName string) *blockchain.Chain {
	// Create a copy to build on
	chain := &blockchain.Chain{
		Blocks:        make([]*blockchain.Block, len(baseChain.Blocks)),
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}

	// Deep copy blocks
	for i, block := range baseChain.Blocks {
		chain.Blocks[i] = block
	}

	// Deep copy account states
	for k, v := range baseChain.AccountStates {
		chain.AccountStates[k] = &blockchain.AccountState{
			Address: v.Address,
			Balance: v.Balance,
			Nonce:   v.Nonce,
		}
	}

	// Add blocks with specified difficulties
	for i, difficulty := range difficulties {
		latestBlock := chain.Blocks[len(chain.Blocks)-1]
		previousHash := blockchain.HashBlockHeader(&latestBlock.Header)

		coinbase := blockchain.Transaction{
			From:   blockchain.PublicKey{},
			To:     minerPubKey,
			Amount: 50,
			Nonce:  0,
		}

		blockParams := blockchain.BlockCreationParams{
			Version:      1,
			PreviousHash: previousHash,
			Height:       uint64(len(chain.Blocks)),
			PreviousWork: latestBlock.Header.TotalWork,
			Coinbase:     coinbase,
			Transactions: []blockchain.Transaction{},
			Timestamp:    uint64(time.Now().Unix()) + uint64(i*1000), // Unique timestamps
			Difficulty:   difficulty,
		}

		newBlock, err := blockchain.NewBlock(blockParams)
		if err != nil {
			t.Fatalf("Failed to create block for %s: %v", chainName, err)
		}

		// Add to chain
		chain.Blocks = append(chain.Blocks, &newBlock)

		// Update account states (add coinbase)
		if toState, ok := chain.AccountStates[minerPubKey]; ok {
			toState.Balance += 50
		} else {
			chain.AccountStates[minerPubKey] = &blockchain.AccountState{
				Address: minerPubKey,
				Balance: 50,
				Nonce:   0,
			}
		}
	}

	return chain
}

func createChainHeadPayload(chain *blockchain.Chain, peerSource string) *p2p.ChainHeadPayload {
	headBlock := chain.Blocks[len(chain.Blocks)-1]
	headHash := blockchain.HashBlockHeader(&headBlock.Header)

	return &p2p.ChainHeadPayload{
		Height:    headBlock.Header.Height,
		HeadHash:  blockchain.EncodeHash(headHash),
		TotalWork: headBlock.Header.TotalWork,
		Header:    headBlock.Header,
	}
}
