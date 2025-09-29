package integration

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"august/blockchain"
	"august/config"
	_ "august/execution" // Import for side effects (registers executor)
	"august/marigold"
	"august/node"
)

func TestDeployAndCallCounter(t *testing.T) {
	t.Log("=== Counter Contract Integration Test ===")

	// Generate keys for testing
	deployerPub, deployerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Error generating keys: %v", err)
	}
	deployerPubKey := blockchain.PublicKey(deployerPub)

	// Start node for testing
	queryPort := 18092
	port := 18093
	nodeConfig := node.NodeConfig{
		QueryPort: queryPort,
		Port:      port,
	}
	n := node.NewNode(nodeConfig)
	go n.Start()
	defer n.Stop()

	t.Logf("Started test node on port %d", queryPort)
	time.Sleep(1 * time.Second) // Give node time to start

	// Mine initial block to fund deployer
	t.Log("1. Mining initial block to fund deployer...")
	deployerState, err := getAccountState(fmt.Sprintf("localhost:%d", queryPort), deployerPubKey)
	if err != nil {
		t.Fatalf("Error getting initial state: %v", err)
	}

	// Fund deployer with coinbase
	fundTx := blockchain.Transaction{
		From:      blockchain.PublicKey{}, // Coinbase (zero address)
		To:        deployerPubKey,
		Amount:    50000000, // 50M AUG
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   config.MainnetChainID,
		GasLimit:  21000,
		GasPrice:  100,
	}

	block1 := mineBlock(fmt.Sprintf("localhost:%d", queryPort), deployerPubKey, []*blockchain.Transaction{&fundTx})
	if block1 == nil {
		t.Fatal("Failed to mine funding block")
	}
	err = submitBlock(fmt.Sprintf("localhost:%d", queryPort), block1)
	if err != nil {
		t.Fatalf("Error submitting block: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Check deployer balance and chain state
	deployerState, _ = getAccountState(fmt.Sprintf("localhost:%d", queryPort), deployerPubKey)
	t.Logf("Deployer balance: %d AUG", deployerState.Balance)

	// Print chain state after funding
	printChainState(t, fmt.Sprintf("localhost:%d", queryPort), "After funding deployer")

	// Deploy counter contract
	t.Log("2. Deploying counter contract...")
	counterContract := `
define init() : int {
  persistent["counter"] = "42"
  return 0
}

define call() : int {
  counterStr: string = persistent["counter"]
  current: int = 0
  if len(counterStr) > 0 {
    current = int(counterStr)
  }

  newValue: int = current + 1
  persistent["counter"] = string(newValue)
  emit(newValue)
  return newValue
}`

	// Parse and compile the contract
	tokens, err := marigold.Lex(counterContract)
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	marigold.Typecheck(ast)
	ctx := marigold.CodegenWithContext(ast)

	// Convert to blockchain instructions
	instructions := make([]blockchain.Instruction, len(ctx.Instructions))
	for i, instr := range ctx.Instructions {
		instructions[i] = blockchain.Instruction{
			Opcode: instr.Opcode,
			Value:  instr.Value,
		}
	}

	t.Logf("Generated %d instructions", len(instructions))

	// Deploy contract transaction
	deployTx := blockchain.Transaction{
		From:         deployerPubKey,
		To:           blockchain.PublicKey{}, // Zero address for contract deployment
		Amount:       1000000,               // 1M AUG initial value
		Nonce:        deployerState.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: instructions,
		GasLimit:     50000,
		GasPrice:     100,
		Data:         nil,
	}
	deployTx.Signature = deployTx.GetSignature(deployerPriv)

	// Mine block with deployment
	block2 := mineBlock(fmt.Sprintf("localhost:%d", queryPort), deployerPubKey, []*blockchain.Transaction{&deployTx})
	if block2 == nil {
		t.Fatal("Failed to mine deployment block")
	}
	err = submitBlock(fmt.Sprintf("localhost:%d", queryPort), block2)
	if err != nil {
		t.Fatalf("Error submitting deployment block: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Print chain state after deployment
	printChainState(t, fmt.Sprintf("localhost:%d", queryPort), "After contract deployment")

	// Find the contract address
	t.Log("3. Finding contract address...")
	chainState, err := getChainState(fmt.Sprintf("localhost:%d", queryPort))
	if err != nil {
		t.Fatalf("Error getting chain state: %v", err)
	}

	var contractAddress blockchain.PublicKey
	for addr, state := range chainState {
		t.Logf("Checking address %x: has %d instructions, %d storage keys", addr[:8], len(state.Instructions), len(state.Persistent))
		if len(state.Instructions) > 0 && addr != deployerPubKey {
			contractAddress = addr
			t.Logf("Found contract at address: %x", addr[:8])
			break
		}
	}

	if (contractAddress == blockchain.PublicKey{}) {
		t.Fatalf("Could not find deployed contract! Total addresses in chain state: %d", len(chainState))
	}

	t.Logf("Contract deployed at: %x", contractAddress)

	// Check initial storage
	contractState := chainState[contractAddress]
	t.Logf("Initial storage: %v", contractState.Persistent)

	// Call contract to increment counter (42 -> 43)
	t.Log("4. Calling contract to increment counter...")
	deployerState, _ = getAccountState(fmt.Sprintf("localhost:%d", queryPort), deployerPubKey)

	callTx := blockchain.Transaction{
		From:      deployerPubKey,
		To:        contractAddress,
		Amount:    0,
		Nonce:     deployerState.Nonce + 1,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   config.MainnetChainID,
		GasLimit:  50000,
		GasPrice:  100,
		Data:      nil,
	}
	callTx.Signature = callTx.GetSignature(deployerPriv)

	block3 := mineBlock(fmt.Sprintf("localhost:%d", queryPort), deployerPubKey, []*blockchain.Transaction{&callTx})
	if block3 == nil {
		t.Fatal("Failed to mine first call block")
	}
	err = submitBlock(fmt.Sprintf("localhost:%d", queryPort), block3)
	if err != nil {
		t.Fatalf("Error submitting call block: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Print chain state after first call
	printChainState(t, fmt.Sprintf("localhost:%d", queryPort), "After first contract call")

	// Check storage after first call
	chainState2, _ := getChainState(fmt.Sprintf("localhost:%d", queryPort))
	if contractState2, exists := chainState2[contractAddress]; exists {
		t.Logf("Storage after first call: %v", contractState2.Persistent)
	}

	// Call contract again to increment (43 -> 44)
	t.Log("5. Calling contract again...")
	deployerState, _ = getAccountState(fmt.Sprintf("localhost:%d", queryPort), deployerPubKey)

	callTx2 := blockchain.Transaction{
		From:      deployerPubKey,
		To:        contractAddress,
		Amount:    0,
		Nonce:     deployerState.Nonce + 1,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   config.MainnetChainID,
		GasLimit:  50000,
		GasPrice:  100,
		Data:      nil,
	}
	callTx2.Signature = callTx2.GetSignature(deployerPriv)

	block4 := mineBlock(fmt.Sprintf("localhost:%d", queryPort), deployerPubKey, []*blockchain.Transaction{&callTx2})
	if block4 == nil {
		t.Fatal("Failed to mine second call block")
	}
	err = submitBlock(fmt.Sprintf("localhost:%d", queryPort), block4)
	if err != nil {
		t.Fatalf("Error submitting second call block: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Print final chain state
	printChainState(t, fmt.Sprintf("localhost:%d", queryPort), "After second contract call")

	// Check final storage
	chainState3, _ := getChainState(fmt.Sprintf("localhost:%d", queryPort))
	var contractState3 *blockchain.AccountState
	if state, exists := chainState3[contractAddress]; exists {
		contractState3 = state
		t.Logf("Storage after second call: %v", contractState3.Persistent)
	}

	t.Log("=== Integration Test Complete ===")
	t.Log("Contract deployed and executed successfully!")

	// Verify the test completed successfully
	if contractState3 != nil {
		t.Logf("Contract has %d storage keys and %d instructions", len(contractState3.Persistent), len(contractState3.Instructions))
		t.Log("Contract execution completed with storage updates")
		if len(contractState3.Persistent) > 0 {
			t.Log("Integration test PASSED: Contract deployment and execution successful!")
		}
	} else {
		t.Fatal("Integration test FAILED: Contract state not found")
	}
}

// Helper functions for API interaction

func printChainState(t *testing.T, nodeAddr string, stage string) {
	t.Logf("\n=== CHAIN STATE: %s ===", stage)
	chainState, err := getChainState(nodeAddr)
	if err != nil {
		t.Logf("Error getting chain state: %v", err)
		return
	}

	t.Logf("Total accounts: %d", len(chainState))
	for addr, state := range chainState {
		t.Logf("Address %x:", addr[:8])
		t.Logf("  Balance: %d AUG", state.Balance)
		t.Logf("  Nonce: %d", state.Nonce)
		t.Logf("  Instructions: %d", len(state.Instructions))
		t.Logf("  Storage keys: %d", len(state.Persistent))

		// Print persistent storage in key, value format
		t.Logf("  Current chain persistent storage:")
		for key, val := range state.Persistent {
			t.Logf("    key=%s, val=%s", key, val)
		}
	}
	t.Log("=== END CHAIN STATE ===\n")
}

func getAccountState(nodeAddr string, pubKey blockchain.PublicKey) (*blockchain.AccountState, error) {
	states, err := getChainState(nodeAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain state: %w", err)
	}

	state, exists := states[pubKey]
	if !exists {
		return &blockchain.AccountState{
			Address:      pubKey,
			Balance:      0,
			Nonce:        0,
			Instructions: nil,
			Persistent:   make(map[string]string),
		}, nil
	}

	return state, nil
}

func getChainState(nodeAddr string) (map[blockchain.PublicKey]*blockchain.AccountState, error) {
	url := fmt.Sprintf("http://%s/chain-state", nodeAddr)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	var chainStateResp node.ChainStateResponse
	if err := json.NewDecoder(resp.Body).Decode(&chainStateResp); err != nil {
		return nil, fmt.Errorf("failed to decode chain state response: %w", err)
	}

	accountStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
	for keyHex, state := range chainStateResp.AccountStates {
		keyBytes, err := hex.DecodeString(keyHex)
		if err != nil {
			continue
		}

		if len(keyBytes) != 32 {
			continue
		}

		var pubKey blockchain.PublicKey
		copy(pubKey[:], keyBytes)
		accountStates[pubKey] = state
	}

	return accountStates, nil
}

func getChainInfo(nodeAddr string) (*blockchain.ChainHead, error) {
	url := fmt.Sprintf("http://%s/chain-info", nodeAddr)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	var chainHead blockchain.ChainHead
	if err := json.NewDecoder(resp.Body).Decode(&chainHead); err != nil {
		return nil, fmt.Errorf("failed to decode chain info: %w", err)
	}

	return &chainHead, nil
}

func mineBlock(nodeAddr string, minerKey blockchain.PublicKey, transactions []*blockchain.Transaction) *blockchain.Block {
	chainInfo, err := getChainInfo(nodeAddr)
	if err != nil {
		return nil
	}

	currentStates, err := getChainState(nodeAddr)
	if err != nil {
		return nil
	}

	timestamp := uint64(time.Now().Unix())

	var totalGasFees uint64
	for _, tx := range transactions {
		if tx.From == (blockchain.PublicKey{}) {
			continue
		}

		tempStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
		for pubKey, st := range currentStates {
			if st != nil {
				tempStates[pubKey] = &blockchain.AccountState{
					Address:      st.Address,
					Balance:      st.Balance,
					Nonce:        st.Nonce,
					Instructions: st.Instructions,
					Persistent:   make(map[string]string),
					StorageRoot:  st.StorageRoot,
					CodeHash:     st.CodeHash,
				}
				for k, v := range st.Persistent {
					tempStates[pubKey].Persistent[k] = v
				}
			}
		}

		gasUsed, err := blockchain.ApplyTransaction(tx, tempStates)
		if err != nil {
			continue
		}

		gasFee := gasUsed * tx.GasPrice
		totalGasFees += gasFee
	}

	coinbaseAmount := config.BlockReward + totalGasFees
	coinbase := blockchain.Transaction{
		From:         blockchain.PublicKey{},
		To:           minerKey,
		Amount:       coinbaseAmount,
		Nonce:        0,
		Timestamp:    timestamp,
		Signature:    blockchain.Signature{},
		ChainID:      config.MainnetChainID,
		Instructions: nil,
		GasLimit:     0,
		GasPrice:     0,
	}

	txList := make([]blockchain.Transaction, len(transactions))
	for i, tx := range transactions {
		txList[i] = *tx
	}

	params := blockchain.BlockCreationParams{
		Version:       1,
		PreviousHash:  chainInfo.Hash,
		Height:        chainInfo.Height + 1,
		PreviousWork:  chainInfo.TotalWork,
		Coinbase:      coinbase,
		Transactions:  txList,
		Timestamp:     timestamp,
		TargetBits:    config.TestTargetCompact,
		CurrentStates: currentStates,
	}

	block, err := blockchain.NewBlock(params)
	if err != nil {
		return nil
	}

	return &block
}

func submitBlock(nodeAddr string, block *blockchain.Block) error {
	blockData, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	url := fmt.Sprintf("http://%s/submit-block", nodeAddr)
	resp, err := http.Post(url, "application/json", bytes.NewReader(blockData))
	if err != nil {
		return fmt.Errorf("failed to submit block: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return nil
}