package tests

import (
	"august/blockchain"
	"august/miner"
	"august/node"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

var tempDir string // Shared temp directory for all tests

// TestMain runs before all tests and sets up temporary directories
func TestMain(m *testing.M) {
	// Create a temporary directory for all tests
	var err error
	tempDir, err = os.MkdirTemp("", "august-storage-tests-*")
	if err != nil {
		panic("Failed to create temp directory: " + err.Error())
	}

	// Run all tests
	code := m.Run()

	// Cleanup: Remove temporary directory
	os.RemoveAll(tempDir)

	// Exit with the same code as the tests
	os.Exit(code)
}

const PORT_START_RANGE = 9370

func createTestNode(idx int) *node.FullNode {

	portAsString := strconv.Itoa(PORT_START_RANGE + idx)

	conf := node.Config{
		Port:      portAsString,
		NodeID:    "test-node-id-" + portAsString,
		SeedPeers: []string{},
		DBName:    filepath.Join(tempDir),
	}

	return node.NewFullNode(conf)
}

func TestBlockchainSaveLoad(t *testing.T) {

	nodeA := createTestNode(0)

	ready := nodeA.Start()
	<-ready
	nodeA.InitializeChain()
	log.Printf("Seed node is ready and accepting connections")

	// Validate chain has only genesis block
	chain, err := nodeA.Store.GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain: %v", err)
	}

	if len(chain.Blocks) != 1 {
		t.Fatalf("Expected exactly 1 block (genesis), got %d", len(chain.Blocks))
	}

	lastBlock := chain.Blocks[len(chain.Blocks)-1]
	if lastBlock.Header.Height != 0 {
		t.Fatalf("Expected genesis block (height 0), got height %d", lastBlock.Header.Height)
	}

	genesisHash := blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header)
	lastBlockHash := blockchain.HashBlockHeader(&lastBlock.Header)
	if genesisHash != lastBlockHash {
		t.Fatalf("Last block is not genesis: expected %x, got %x", genesisHash[:8], lastBlockHash[:8])
	}

	log.Printf("Validated: chain contains only genesis block (height 0)")

	// Mine a block
	chainHead := nodeA.GetChainHead()

	// Create mining parameters
	params := miner.BlockCreationParams{
		Version:      1,
		PreviousHash: chainHead.Hash,
		Height:       chainHead.Height + 1,
		PreviousWork: chainHead.TotalWork,
		Coinbase: blockchain.Transaction{
			From:      blockchain.PublicKey{}, // Empty for coinbase
			To:        blockchain.FirstUser,   // Mine to first user
			Amount:    blockchain.BlockReward,
			Fee:       0,
			Nonce:     0,
			Timestamp: uint64(time.Now().Unix()),
			Signature: blockchain.Signature{}, // Empty for coinbase
		},
		Transactions: []blockchain.Transaction{}, // No other transactions
		Timestamp:    uint64(time.Now().Unix()),
		TargetBits:   blockchain.TestTargetCompact, // Super easy difficulty for testing
	}

	// Mine the block
	log.Printf("Mining block at height %d...", params.Height)
	block, err := miner.NewBlock(params)
	if err != nil {
		t.Fatalf("Failed to mine block: %v", err)
	}

	blockHash := blockchain.HashBlockHeader(&block.Header)
	log.Printf("Mined block %x (height %d, nonce %d)", blockHash[:8], block.Header.Height, block.Header.Nonce)

	// Submit block directly (no HTTP API)
	done := nodeA.ProcessBlock(&block)
	<-done // Wait for processing to complete

	log.Printf("Block submitted and processed successfully!")

	// Verify chain updated
	newChainHead := nodeA.GetChainHead()
	if newChainHead.Height != chainHead.Height+1 {
		t.Fatalf("Chain height not updated: expected %d, got %d", chainHead.Height+1, newChainHead.Height)
	}

	log.Printf("Chain updated successfully - new height: %d", newChainHead.Height)

	// Stop NodeA
	err = nodeA.Stop()
	if err != nil {
		t.Fatalf("Failed to stop nodeA: %v", err)
	}
	log.Printf("NodeA stopped")

	// Create NodeB with same database to test persistence
	nodeB := createTestNode(1)                // Different port but same temp directory
	nodeB.Config.DBName = nodeA.Config.DBName // Use same database path

	// Start NodeB
	readyB := nodeB.Start()
	<-readyB
	err = nodeB.InitializeChain()
	if err != nil {
		t.Fatalf("Failed to initialize NodeB chain: %v", err)
	}
	log.Printf("NodeB started and initialized")

	// Verify NodeB loaded the mined block from persistence
	chainB, err := nodeB.Store.GetChain()
	if err != nil {
		t.Fatalf("Failed to get NodeB chain: %v", err)
	}

	if len(chainB.Blocks) != 2 {
		t.Fatalf("NodeB should have 2 blocks (genesis + mined), got %d", len(chainB.Blocks))
	}

	// Verify the mined block is there
	minedBlock := chainB.Blocks[1]
	if minedBlock.Header.Height != 1 {
		t.Fatalf("Expected mined block at height 1, got %d", minedBlock.Header.Height)
	}

	minedBlockHash := blockchain.HashBlockHeader(&minedBlock.Header)
	if minedBlockHash != blockHash {
		t.Fatalf("Mined block hash mismatch: expected %x, got %x", blockHash[:8], minedBlockHash[:8])
	}

	chainHeadB := nodeB.GetChainHead()
	if chainHeadB.Height != 1 {
		t.Fatalf("NodeB chain head should be height 1, got %d", chainHeadB.Height)
	}

	log.Printf("SUCCESS: NodeB loaded mined block from persistence - height: %d, hash: %x",
		chainHeadB.Height, minedBlockHash[:8])

	// Clean up NodeB
	err = nodeB.Stop()
	if err != nil {
		t.Logf("Warning: Failed to stop nodeB: %v", err)
	}
}
