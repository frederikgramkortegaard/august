package tests

import (
	"august/blockchain"
	"august/config"
	"august/mempool"
	"august/miner"
	"august/node"
	"august/utils"
	"crypto/ed25519"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

var tempDir string

// TestMain runs before all tests and sets up temporary directories
func TestMain(m *testing.M) {
	var err error
	tempDir, err = os.MkdirTemp("", "august-mempool-tests-*")
	if err != nil {
		panic("Failed to create temp directory: " + err.Error())
	}

	// Run all tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(tempDir)
	os.Exit(code)
}

const MEMPOOL_PORT_START = 9400

func createMempoolTestNode(idx int) *node.FullNode {
	portAsString := strconv.Itoa(MEMPOOL_PORT_START + idx)

	conf := node.NodeConfig{
		Port:           portAsString,
		NodeID:         "mempool-test-node-" + portAsString,
		SeedPeers:      []string{},
		DBName:         filepath.Join(tempDir, "node-"+portAsString+".db"),
		MaxMempoolSize: 10, // Small for testing
		MempoolExpiry:  5 * time.Minute,
	}

	return node.NewFullNode(conf)
}

// generateKeyPair creates a test keypair
func generateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic("Failed to generate keypair: " + err.Error())
	}
	return pub, priv
}

// createTestTransaction creates a valid test transaction
func createTestTransaction(from, to blockchain.PublicKey, amount, fee uint64, nonce uint64, privateKey ed25519.PrivateKey) blockchain.Transaction {
	tx := blockchain.Transaction{
		From:      from,
		To:        to,
		Amount:    amount,
		GasLimit:  25000,  // 25k gas limit (enough for basic transfer)
		GasPrice:  fee,    // Fee is now gas price directly (simplified for tests)
		Nonce:     nonce,
		Timestamp: uint64(time.Now().Unix()),
	}

	// Sign the transaction
	tx.Signature = tx.GetSignature(privateKey)
	return tx
}

func TestMempoolBasicOperations(t *testing.T) {
	// Create node
	nodeA := createMempoolTestNode(0)

	// Start and initialize
	ready := nodeA.Start()
	<-ready
	err := nodeA.InitializeChain()
	if err != nil {
		t.Fatalf("Failed to initialize chain: %v", err)
	}

	log.Printf("Node started and chain initialized")

	// Test mempool is empty initially
	if nodeA.Mempool.Size() != 0 {
		t.Fatalf("Expected empty mempool, got size %d", nodeA.Mempool.Size())
	}

	// Generate test keypairs
	pubA, privA := generateKeyPair()
	pubB, privB := generateKeyPair()
	_, _ = pubB, privB // Suppress unused warning

	var keyA, keyB blockchain.PublicKey
	copy(keyA[:], pubA)
	copy(keyB[:], pubB)

	// Get initial chain state for testing
	chain, err := nodeA.Store.GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain: %v", err)
	}

	// Set up some initial balances (simulate mining rewards)
	chain.AccountStates[keyA] = &blockchain.AccountState{
		Balance: 10 * config.AUG, // 10 AUG = 10M Leaf units
		Nonce:   0,
	}

	// Save the updated chain state
	err = nodeA.Store.ReplaceChain(chain)
	if err != nil {
		t.Fatalf("Failed to save chain state: %v", err)
	}

	// Save the updated chain state
	err = nodeA.Store.ReplaceChain(chain)
	if err != nil {
		t.Fatalf("Failed to save chain state: %v", err)
	}

	// Test 1: Add valid transaction
	tx1 := createTestTransaction(keyA, keyB, 100, 10, 1, privA)

	// Submit transaction through node
	err = nodeA.SubmitTransaction(&tx1)
	if err != nil {
		t.Fatalf("Failed to submit valid transaction: %v", err)
	}

	// Verify it was added to mempool
	if nodeA.Mempool.Size() != 1 {
		t.Fatalf("Expected mempool size 1, got %d", nodeA.Mempool.Size())
	}

	log.Printf("Successfully added transaction to mempool")

	// Test 2: Try to add duplicate transaction (should fail)
	err = nodeA.SubmitTransaction(&tx1)
	if err == nil {
		t.Fatalf("Expected duplicate transaction to be rejected")
	}

	// Verify mempool size unchanged
	if nodeA.Mempool.Size() != 1 {
		t.Fatalf("Expected mempool size still 1 after duplicate, got %d", nodeA.Mempool.Size())
	}

	// Test 3: Create different user for second transaction to avoid nonce conflicts
	pubC, privC := generateKeyPair()
	var keyC blockchain.PublicKey
	copy(keyC[:], pubC)

	// Set up balance for second user
	chain.AccountStates[keyC] = &blockchain.AccountState{
		Balance: 10 * config.AUG, // 10 AUG = 10M Leaf units
		Nonce:   0,
	}

	// Save the updated chain state
	err = nodeA.Store.ReplaceChain(chain)
	if err != nil {
		t.Fatalf("Failed to save chain state: %v", err)
	}

	// Add transaction with higher fee from different user
	tx2 := createTestTransaction(keyC, keyB, 50, 20, 1, privC)
	err = nodeA.SubmitTransaction(&tx2)
	if err != nil {
		t.Fatalf("Failed to submit second transaction: %v", err)
	}

	if nodeA.Mempool.Size() != 2 {
		t.Fatalf("Expected mempool size 2, got %d", nodeA.Mempool.Size())
	}

	// Test 4: Get transactions (should be ordered by fee - highest first)
	transactions := nodeA.Mempool.GetTransactions(10)
	if len(transactions) != 2 {
		t.Fatalf("Expected 2 transactions, got %d", len(transactions))
	}

	// Higher fee transaction should come first
	expectedFee0 := transactions[0].GasLimit * transactions[0].GasPrice
	if expectedFee0 != 500000 { // 25000 * 20
		t.Fatalf("Expected first transaction to have fee 500000, got %d", expectedFee0)
	}
	expectedFee1 := transactions[1].GasLimit * transactions[1].GasPrice
	if expectedFee1 != 250000 { // 25000 * 10
		t.Fatalf("Expected second transaction to have fee 250000, got %d", expectedFee1)
	}

	log.Printf("Transaction fee ordering works correctly")

	// Stop node
	err = nodeA.Stop()
	if err != nil {
		t.Fatalf("Failed to stop node: %v", err)
	}

	log.Printf("SUCCESS: Mempool basic operations test passed")
}

func TestMempoolWithBlockProcessing(t *testing.T) {
	// Create node
	nodeA := createMempoolTestNode(1)

	// Start and initialize
	ready := nodeA.Start()
	<-ready
	err := nodeA.InitializeChain()
	if err != nil {
		t.Fatalf("Failed to initialize chain: %v", err)
	}

	// Generate test keypairs
	pubA, privA := generateKeyPair()
	pubB, _ := generateKeyPair()

	var keyA, keyB blockchain.PublicKey
	copy(keyA[:], pubA)
	copy(keyB[:], pubB)

	// Set up initial balance
	chain, err := nodeA.Store.GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain: %v", err)
	}
	chain.AccountStates[keyA] = &blockchain.AccountState{
		Balance: 10 * config.AUG, // 10 AUG = 10M Leaf units
		Nonce:   0,
	}

	// Save the updated chain state
	err = nodeA.Store.ReplaceChain(chain)
	if err != nil {
		t.Fatalf("Failed to save chain state: %v", err)
	}

	// Create another user for second transaction
	pubC, privC := generateKeyPair()
	var keyC blockchain.PublicKey
	copy(keyC[:], pubC)

	chain.AccountStates[keyC] = &blockchain.AccountState{
		Balance: 10 * config.AUG, // 10 AUG = 10M Leaf units
		Nonce:   0,
	}

	// Save the updated chain state
	err = nodeA.Store.ReplaceChain(chain)
	if err != nil {
		t.Fatalf("Failed to save chain state: %v", err)
	}

	// Add transactions to mempool from different users
	tx1 := createTestTransaction(keyA, keyB, 100, 10, 1, privA)
	tx2 := createTestTransaction(keyC, keyB, 200, 15, 1, privC)

	err = nodeA.SubmitTransaction(&tx1)
	if err != nil {
		t.Fatalf("Failed to submit tx1: %v", err)
	}
	err = nodeA.SubmitTransaction(&tx2)
	if err != nil {
		t.Fatalf("Failed to submit tx2: %v", err)
	}

	// Verify both transactions in mempool
	if nodeA.Mempool.Size() != 2 {
		t.Fatalf("Expected 2 transactions in mempool, got %d", nodeA.Mempool.Size())
	}

	log.Printf("Added 2 transactions to mempool")

	// Create and mine a block containing one of the transactions
	chainHead := nodeA.GetChainHead()
	currentChain, err := nodeA.Store.GetChain()
	if err != nil {
		t.Fatalf("Failed to get current chain state: %v", err)
	}

	// Use the miner package to create a proper block
	params := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  chainHead.Hash,
		Height:        chainHead.Height + 1,
		PreviousWork:  chainHead.TotalWork,
		CurrentStates: currentChain.AccountStates, // Add current states for gas calculation
		Coinbase: func() blockchain.Transaction {
			// Get actual gas used by applying transaction first
			tempStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
			for pubKey, state := range currentChain.AccountStates {
				if state != nil {
					tempStates[pubKey] = &blockchain.AccountState{
						Balance:      state.Balance,
						Nonce:        state.Nonce,
						Instructions: state.Instructions,
						Persistent:   make(map[string]string),
						StorageRoot:  state.StorageRoot,
						CodeHash:     state.CodeHash,
					}
					for k, v := range state.Persistent {
						tempStates[pubKey].Persistent[k] = v
					}
				}
			}
			actualGasUsed, err := blockchain.ApplyTransaction(&tx1, tempStates)
			if err != nil {
				t.Fatalf("Failed to apply transaction for gas calculation: %v", err)
			}
			txFee := actualGasUsed * tx1.GasPrice
			coinbaseAmount, err := utils.SafeAdd(config.BlockReward, txFee)
			if err != nil {
				panic(fmt.Sprintf("coinbase amount overflow in test: %v", err))
			}
			return blockchain.Transaction{
				From:      blockchain.PublicKey{}, // Empty for coinbase
				To:        blockchain.FirstUser,   // Mine to first user
				Amount:    coinbaseAmount,         // Block reward + transaction fees
				GasLimit:  0,  // Coinbase transactions use no gas
				GasPrice:  0,
				Nonce:     0,
				Timestamp: uint64(time.Now().Unix()),
				Signature: blockchain.Signature{}, // Empty for coinbase
			}
		}(),
		Transactions: []blockchain.Transaction{tx1}, // Include tx1 in the block
		Timestamp:    uint64(time.Now().Unix()),
		TargetBits:   config.TestTargetCompact, // Easy difficulty for testing
	}

	// Mine the block
	block, err := miner.NewBlock(params)
	if err != nil {
		t.Fatalf("Failed to mine block: %v", err)
	}

	// Process the block
	done := nodeA.ProcessBlock(&block)
	<-done

	// Wait a bit for async mempool cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify that tx1 was removed from mempool but tx2 remains
	if nodeA.Mempool.Size() != 1 {
		t.Fatalf("Expected 1 transaction remaining in mempool after block, got %d", nodeA.Mempool.Size())
	}

	// Verify the remaining transaction is tx2
	remaining := nodeA.Mempool.GetTransactions(1)
	expectedRemaining := remaining[0].GasLimit * remaining[0].GasPrice
	if len(remaining) != 1 || expectedRemaining != 375000 { // 25000 * 15
		t.Fatalf("Expected remaining transaction to have fee 375000, got %d", expectedRemaining)
	}

	log.Printf("SUCCESS: Mempool correctly cleaned up after block processing")

	// Stop node
	err = nodeA.Stop()
	if err != nil {
		t.Fatalf("Failed to stop node: %v", err)
	}
}

func TestMempoolPersistenceAndAPI(t *testing.T) {
	// Create node with miner API
	nodeA := createMempoolTestNode(2)
	nodeA.Config.QueryPort = strconv.Itoa(MEMPOOL_PORT_START + 100)

	// Start and initialize
	ready := nodeA.Start()
	<-ready
	err := nodeA.InitializeChain()
	if err != nil {
		t.Fatalf("Failed to initialize chain: %v", err)
	}

	// Give the HTTP server time to start
	time.Sleep(100 * time.Millisecond)

	// Generate test data
	pubA, privA := generateKeyPair()
	pubB, _ := generateKeyPair()

	var keyA, keyB blockchain.PublicKey
	copy(keyA[:], pubA)
	copy(keyB[:], pubB)

	// Set up balance
	chain, err := nodeA.Store.GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain: %v", err)
	}
	chain.AccountStates[keyA] = &blockchain.AccountState{
		Balance: 10 * config.AUG, // 10 AUG = 10M Leaf units
		Nonce:   0,
	}

	// Save the updated chain state
	err = nodeA.Store.ReplaceChain(chain)
	if err != nil {
		t.Fatalf("Failed to save chain state: %v", err)
	}

	// Create another user for second transaction
	pubC, privC := generateKeyPair()
	var keyC blockchain.PublicKey
	copy(keyC[:], pubC)

	chain.AccountStates[keyC] = &blockchain.AccountState{
		Balance: 10 * config.AUG, // 10 AUG = 10M Leaf units
		Nonce:   0,
	}

	// Save the updated chain state
	err = nodeA.Store.ReplaceChain(chain)
	if err != nil {
		t.Fatalf("Failed to save chain state: %v", err)
	}

	// Add transactions from different users
	tx1 := createTestTransaction(keyA, keyB, 100, 25, 1, privA) // High fee
	tx2 := createTestTransaction(keyC, keyB, 50, 5, 1, privC)   // Low fee

	err = nodeA.SubmitTransaction(&tx1)
	if err != nil {
		t.Fatalf("Failed to submit tx1: %v", err)
	}
	err = nodeA.SubmitTransaction(&tx2)
	if err != nil {
		t.Fatalf("Failed to submit tx2: %v", err)
	}

	// Verify mempool contains transactions
	if nodeA.Mempool.Size() != 2 {
		t.Fatalf("Expected 2 transactions in mempool, got %d", nodeA.Mempool.Size())
	}

	// Test HTTP API by getting transactions
	transactions := nodeA.Mempool.GetTransactions(10)
	if len(transactions) != 2 {
		t.Fatalf("Expected 2 transactions from API, got %d", len(transactions))
	}

	// Verify fee ordering (highest first)
	fee0 := transactions[0].GasLimit * transactions[0].GasPrice
	if fee0 != 625000 { // 25000 * 25
		t.Fatalf("Expected first transaction fee 625000, got %d", fee0)
	}
	fee1 := transactions[1].GasLimit * transactions[1].GasPrice
	if fee1 != 125000 { // 25000 * 5
		t.Fatalf("Expected second transaction fee 125000, got %d", fee1)
	}

	log.Printf("SUCCESS: Mempool API returns correctly ordered transactions")

	// Stop node
	err = nodeA.Stop()
	if err != nil {
		t.Fatalf("Failed to stop node: %v", err)
	}
}

func TestMempoolSizeLimits(t *testing.T) {
	// Create node with very small mempool
	nodeA := createMempoolTestNode(3)
	// Set the configuration before starting the node
	nodeA.Config.MaxMempoolSize = 2

	// Recreate the mempool with the new configuration
	mempoolConfig := mempool.MempoolConfig{
		MaxSize:   nodeA.Config.MaxMempoolSize,
		MaxExpiry: nodeA.Config.MempoolExpiry,
	}
	nodeA.Mempool = mempool.NewMempool(mempoolConfig)

	ready := nodeA.Start()
	<-ready
	err := nodeA.InitializeChain()
	if err != nil {
		t.Fatalf("Failed to initialize chain: %v", err)
	}

	// Set up test accounts - create 3 different users for 3 transactions
	pubA, privA := generateKeyPair()
	pubB, _ := generateKeyPair()
	pubC, privC := generateKeyPair()
	pubD, privD := generateKeyPair()

	var keyA, keyB, keyC, keyD blockchain.PublicKey
	copy(keyA[:], pubA)
	copy(keyB[:], pubB)
	copy(keyC[:], pubC)
	copy(keyD[:], pubD)

	chain, err := nodeA.Store.GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain: %v", err)
	}

	// Set up balances for all users
	chain.AccountStates[keyA] = &blockchain.AccountState{
		Balance: 10 * config.AUG, // 10 AUG = 10M Leaf units
		Nonce:   0,
	}
	chain.AccountStates[keyC] = &blockchain.AccountState{
		Balance: 10 * config.AUG, // 10 AUG = 10M Leaf units
		Nonce:   0,
	}
	chain.AccountStates[keyD] = &blockchain.AccountState{
		Balance: 10 * config.AUG, // 10 AUG = 10M Leaf units
		Nonce:   0,
	}

	// Save the updated chain state
	err = nodeA.Store.ReplaceChain(chain)
	if err != nil {
		t.Fatalf("Failed to save chain state: %v", err)
	}

	// Add transactions with different fees from different users
	tx1 := createTestTransaction(keyA, keyB, 100, 5, 1, privA)  // Low fee
	tx2 := createTestTransaction(keyC, keyB, 100, 10, 1, privC) // Medium fee
	tx3 := createTestTransaction(keyD, keyB, 100, 20, 1, privD) // High fee

	// Add first two transactions
	err = nodeA.SubmitTransaction(&tx1)
	if err != nil {
		t.Fatalf("Failed to submit tx1: %v", err)
	}
	err = nodeA.SubmitTransaction(&tx2)
	if err != nil {
		t.Fatalf("Failed to submit tx2: %v", err)
	}

	if nodeA.Mempool.Size() != 2 {
		t.Fatalf("Expected mempool size 2, got %d", nodeA.Mempool.Size())
	}

	// Add third transaction (should evict lowest fee transaction)
	err = nodeA.SubmitTransaction(&tx3)
	if err != nil {
		t.Fatalf("Failed to submit tx3: %v", err)
	}

	// Mempool should still be size 2
	if nodeA.Mempool.Size() != 2 {
		t.Fatalf("Expected mempool size 2 after eviction, got %d", nodeA.Mempool.Size())
	}

	// Check that highest fee transactions remain
	transactions := nodeA.Mempool.GetTransactions(10)
	if len(transactions) != 2 {
		t.Fatalf("Expected 2 transactions after eviction, got %d", len(transactions))
	}

	// Should have transactions with fees 500000 and 250000 (tx1 with fee 125000 should be evicted)
	fees := []uint64{transactions[0].GasLimit * transactions[0].GasPrice, transactions[1].GasLimit * transactions[1].GasPrice}
	expectedFees := []uint64{500000, 250000} // 25000 * 20, 25000 * 10

	if fees[0] != expectedFees[0] || fees[1] != expectedFees[1] {
		t.Fatalf("Expected fees [500000, 250000], got %v", fees)
	}

	log.Printf("SUCCESS: Mempool correctly evicts low-fee transactions when full")

	err = nodeA.Stop()
	if err != nil {
		t.Fatalf("Failed to stop node: %v", err)
	}
}