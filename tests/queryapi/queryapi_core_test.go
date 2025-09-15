package tests

import (
	"august/blockchain"
	"august/miner"
	"august/node"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	tempDir, err = os.MkdirTemp("", "august-queryapi-tests-*")
	if err != nil {
		panic("Failed to create temp directory: " + err.Error())
	}

	// Run all tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(tempDir)
	os.Exit(code)
}

const QUERYAPI_PORT_START = 9500

func createQueryAPITestNode(idx int) *node.FullNode {
	portAsString := strconv.Itoa(QUERYAPI_PORT_START + idx)
	queryPortAsString := strconv.Itoa(QUERYAPI_PORT_START + 100 + idx)

	conf := node.Config{
		Port:           portAsString,
		NodeID:         "queryapi-test-node-" + portAsString,
		SeedPeers:      []string{},
		DBName:         filepath.Join(tempDir, "node-"+portAsString+".db"),
		QueryPort:      queryPortAsString,
		MaxMempoolSize: 100,
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

// httpGet performs HTTP GET request
func httpGet(url string) (map[string]interface{}, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Debug: log response if it's not JSON
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v\nResponse body: %s", err, string(body))
	}
	return result, err
}

func TestQueryAPIBasicEndpoints(t *testing.T) {
	// Create and start node with query API
	nodeA := createQueryAPITestNode(0)

	ready := nodeA.Start()
	<-ready
	err := nodeA.InitializeChain()
	if err != nil {
		t.Fatalf("Failed to initialize chain: %v", err)
	}

	// Give the HTTP server time to start
	time.Sleep(200 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%s", nodeA.Config.QueryPort)

	log.Printf("Testing Query API endpoints on %s", baseURL)

	// Test 1: Chain info endpoint
	resp, err := httpGet(baseURL + "/chain-info")
	if err != nil {
		t.Fatalf("Failed to get chain info: %v", err)
	}

	if resp["height"].(float64) != 0 {
		t.Fatalf("Expected genesis height 0, got %v", resp["height"])
	}

	log.Printf("✓ /chain-info endpoint works")

	// Test 2: Mempool endpoint (should be empty)
	resp, err = httpGet(baseURL + "/mempool")
	if err != nil {
		t.Fatalf("Failed to get mempool: %v", err)
	}

	if resp["count"].(float64) != 0 {
		t.Fatalf("Expected empty mempool, got count %v", resp["count"])
	}

	log.Printf("✓ /mempool endpoint works")

	// Test 3: Balance endpoint (unknown address should return 0)
	unknownAddr := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	resp, err = httpGet(baseURL + "/balance/" + unknownAddr)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}

	if resp["exists"].(bool) != false {
		t.Fatalf("Expected unknown address to not exist")
	}
	if resp["balance"].(float64) != 0 {
		t.Fatalf("Expected balance 0 for unknown address, got %v", resp["balance"])
	}

	log.Printf("✓ /balance endpoint works for unknown address")

	// Test 4: Transaction endpoint (unknown hash should return not found)
	unknownHash := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	// First check the length - this should be a valid 32-byte hash
	hashBytes, _ := hex.DecodeString(unknownHash)
	if len(hashBytes) != 32 {
		t.Fatalf("Test hash should be 32 bytes, got %d", len(hashBytes))
	}

	resp, err = httpGet(baseURL + "/transaction/" + unknownHash)
	if err != nil {
		t.Fatalf("Failed to get transaction: %v", err)
	}

	if resp["found"].(bool) != false {
		t.Fatalf("Expected unknown transaction to not be found")
	}

	log.Printf("✓ /transaction endpoint works for unknown hash")

	// Test 5: Block endpoint (unknown hash should return not found)
	resp, err = httpGet(baseURL + "/block/" + unknownHash)
	if err != nil {
		t.Fatalf("Failed to get block: %v", err)
	}

	if resp["found"].(bool) != false {
		t.Fatalf("Expected unknown block to not be found")
	}

	log.Printf("✓ /block endpoint works for unknown hash")

	// Stop node
	err = nodeA.Stop()
	if err != nil {
		t.Fatalf("Failed to stop node: %v", err)
	}

	log.Printf("SUCCESS: All basic Query API endpoints work correctly")
}

func TestQueryAPIWithData(t *testing.T) {
	// Create and start node
	nodeA := createQueryAPITestNode(1)

	ready := nodeA.Start()
	<-ready
	err := nodeA.InitializeChain()
	if err != nil {
		t.Fatalf("Failed to initialize chain: %v", err)
	}

	// Give the HTTP server time to start
	time.Sleep(200 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%s", nodeA.Config.QueryPort)

	// Generate test keypairs
	pubA, privA := generateKeyPair()
	pubB, _ := generateKeyPair()

	var keyA, keyB blockchain.PublicKey
	copy(keyA[:], pubA)
	copy(keyB[:], pubB)

	// Set up initial balance in chain state
	chain, err := nodeA.Store.GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain: %v", err)
	}

	chain.AccountStates[keyA] = &blockchain.AccountState{
		Balance: 20 * blockchain.AUG, // 20 AUG = 20M Leaf units, enough for transaction + gas
		Nonce:   0,
	}

	// Save the updated chain state
	err = nodeA.Store.ReplaceChain(chain)
	if err != nil {
		t.Fatalf("Failed to save chain state: %v", err)
	}

	// Test balance endpoint with actual data
	keyAHex := hex.EncodeToString(keyA[:])
	resp, err := httpGet(baseURL + "/balance/" + keyAHex)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}

	if resp["exists"].(bool) != true {
		t.Fatalf("Expected keyA to exist")
	}
	expectedBalance := float64(20 * blockchain.AUG)
	if resp["balance"].(float64) != expectedBalance {
		t.Fatalf("Expected balance %v, got %v", expectedBalance, resp["balance"])
	}

	log.Printf("✓ /balance endpoint correctly shows account with balance %v", expectedBalance)

	// Create and add a transaction to mempool
	tx := blockchain.Transaction{
		From:      keyA,
		To:        keyB,
		Amount:    100 * blockchain.Leaf,
		GasLimit:  25000,  // 25k gas limit
		GasPrice:  400,    // 400 leaf per gas (25k * 400 = 10M leaf total)
		Nonce:     1,
		Timestamp: uint64(time.Now().Unix()),
	}

	// Sign the transaction
	blockchain.SignTransaction(&tx, privA)

	// Add to mempool
	err = nodeA.SubmitTransaction(&tx)
	if err != nil {
		t.Fatalf("Failed to submit transaction: %v", err)
	}

	// Test mempool endpoint with transaction
	resp, err = httpGet(baseURL + "/mempool")
	if err != nil {
		t.Fatalf("Failed to get mempool: %v", err)
	}

	if resp["count"].(float64) != 1 {
		t.Fatalf("Expected 1 transaction in mempool, got %v", resp["count"])
	}

	log.Printf("✓ /mempool endpoint shows transaction in pool")

	// Test transaction endpoint with pending transaction
	txHash := blockchain.HashTransaction(&tx)
	txHashHex := hex.EncodeToString(txHash[:])

	resp, err = httpGet(baseURL + "/transaction/" + txHashHex)
	if err != nil {
		t.Fatalf("Failed to get transaction: %v", err)
	}

	if resp["found"].(bool) != true {
		t.Fatalf("Expected transaction to be found")
	}
	if resp["status"].(string) != "pending" {
		t.Fatalf("Expected transaction status 'pending', got %v", resp["status"])
	}

	log.Printf("✓ /transaction endpoint finds pending transaction")

	// Get actual gas used by applying transaction first
	chain, err = nodeA.GetChain()
	if err != nil {
		t.Fatalf("Failed to get chain: %v", err)
	}

	// Create temp states for gas calculation
	tempStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
	for pubKey, state := range chain.AccountStates {
		if state != nil {
			tempStates[pubKey] = &blockchain.AccountState{
				Address:      state.Address,
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

	actualGasUsed, err := blockchain.ApplyTransaction(&tx, tempStates)
	if err != nil {
		t.Fatalf("Failed to apply transaction for gas calculation: %v", err)
	}
	t.Logf("Actual gas used for transaction: %d", actualGasUsed)

	// Mine a block containing the transaction
	chainHead := nodeA.GetChainHead()

	params := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  chainHead.Hash,
		Height:        chainHead.Height + 1,
		PreviousWork:  chainHead.TotalWork,
		CurrentStates: chain.AccountStates,
		Coinbase: func() blockchain.Transaction {
			txFee := actualGasUsed * tx.GasPrice
			coinbaseAmount, err := blockchain.SafeAdd(blockchain.BlockReward, txFee)
			if err != nil {
				panic(fmt.Sprintf("coinbase amount overflow in test: %v", err))
			}
			return blockchain.Transaction{
				From:      blockchain.PublicKey{},
				To:        blockchain.FirstUser,
				Amount:    coinbaseAmount,
				GasLimit:  0,  // Coinbase transactions use no gas
				GasPrice:  0,
				Nonce:     0,
				Timestamp: uint64(time.Now().Unix()),
				Signature: blockchain.Signature{},
			}
		}(),
		Transactions: []blockchain.Transaction{tx},
		Timestamp:    uint64(time.Now().Unix()),
		TargetBits:   blockchain.TestTargetCompact,
	}

	block, err := miner.NewBlock(params)
	if err != nil {
		t.Fatalf("Failed to mine block: %v", err)
	}

	// Process the block
	done := nodeA.ProcessBlock(&block)
	<-done

	// Wait for mempool cleanup
	time.Sleep(100 * time.Millisecond)

	log.Printf("✓ Block mined and processed")

	// Test transaction endpoint with confirmed transaction
	resp, err = httpGet(baseURL + "/transaction/" + txHashHex)
	if err != nil {
		t.Fatalf("Failed to get confirmed transaction: %v", err)
	}

	if resp["found"].(bool) != true {
		t.Fatalf("Expected confirmed transaction to be found")
	}
	if _, hasPending := resp["status"]; hasPending {
		t.Fatalf("Expected confirmed transaction to not have pending status")
	}
	if resp["block_height"].(float64) != 1 {
		t.Fatalf("Expected transaction at block height 1, got %v", resp["block_height"])
	}

	log.Printf("✓ /transaction endpoint finds confirmed transaction with block info")

	// Test block endpoint with mined block
	blockHash := blockchain.HashBlockHeader(&block.Header)
	blockHashHex := hex.EncodeToString(blockHash[:])

	resp, err = httpGet(baseURL + "/block/" + blockHashHex)
	if err != nil {
		t.Fatalf("Failed to get block: %v", err)
	}

	if resp["found"].(bool) != true {
		t.Fatalf("Expected block to be found")
	}
	if resp["height"].(float64) != 1 {
		t.Fatalf("Expected block height 1, got %v", resp["height"])
	}
	if resp["tx_count"].(float64) != 2 { // 1 regular tx + 1 coinbase
		t.Fatalf("Expected 2 transactions in block, got %v", resp["tx_count"])
	}

	log.Printf("✓ /block endpoint finds mined block with transaction count")

	// Test mempool is now empty
	resp, err = httpGet(baseURL + "/mempool")
	if err != nil {
		t.Fatalf("Failed to get mempool: %v", err)
	}

	if resp["count"].(float64) != 0 {
		t.Fatalf("Expected empty mempool after block, got count %v", resp["count"])
	}

	log.Printf("✓ /mempool endpoint shows transaction removed after mining")

	// Stop node
	err = nodeA.Stop()
	if err != nil {
		t.Fatalf("Failed to stop node: %v", err)
	}

	log.Printf("SUCCESS: Query API correctly handles real blockchain data")
}

func TestQueryAPIErrorHandling(t *testing.T) {
	// Create and start node
	nodeA := createQueryAPITestNode(2)

	ready := nodeA.Start()
	<-ready
	err := nodeA.InitializeChain()
	if err != nil {
		t.Fatalf("Failed to initialize chain: %v", err)
	}

	// Give the HTTP server time to start
	time.Sleep(200 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%s", nodeA.Config.QueryPort)

	// Test invalid address format
	resp, err := http.Get(baseURL + "/balance/invalid")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected 400 for invalid address, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	log.Printf("✓ /balance endpoint correctly rejects invalid address format")

	// Test invalid transaction hash format
	resp, err = http.Get(baseURL + "/transaction/invalid")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected 400 for invalid hash, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	log.Printf("✓ /transaction endpoint correctly rejects invalid hash format")

	// Test invalid block hash format
	resp, err = http.Get(baseURL + "/block/invalid")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected 400 for invalid hash, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	log.Printf("✓ /block endpoint correctly rejects invalid hash format")

	// Test wrong HTTP methods
	resp, err = http.Post(baseURL+"/chain-info", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to make POST request: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("Expected 405 for POST to chain-info, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	log.Printf("✓ Endpoints correctly reject wrong HTTP methods")

	// Stop node
	err = nodeA.Stop()
	if err != nil {
		t.Fatalf("Failed to stop node: %v", err)
	}

	log.Printf("SUCCESS: Query API properly handles errors and invalid input")
}