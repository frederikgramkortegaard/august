package contracts

import (
	"august/blockchain"
	"august/config"
	_ "august/execution"
	"august/marigold"
	"august/node"
	"august/types"
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestPersistentStorage(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	deployerPubKey := blockchain.PublicKey(pub)

	queryPort := 18082
	port := 18083
	nodeConfig := node.NodeConfig{
		QueryPort: queryPort,
		Port:      port,
	}
	n := node.NewNode(nodeConfig)
	go n.Start()
	defer n.Stop()

	time.Sleep(500 * time.Millisecond)

	nodeAddr := fmt.Sprintf("localhost:%d", queryPort)

	t.Log("Mining initial block to fund deployer account")
	block1 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{})
	submitBlock(t, nodeAddr, block1)
	time.Sleep(200 * time.Millisecond)

	balance := getBalance(t, nodeAddr, deployerPubKey)
	if balance == 0 {
		t.Fatalf("Deployer has no balance after mining")
	}
	t.Logf("Deployer balance: %d", balance)

	// Contract that stores and retrieves values in persistent storage
	code := `
	define init() : int {
		persistent["counter"] = "0"
		persistent["owner"] = "deployer"
		emit(42)
		return 1
	}

	define call() : int {
		// Increment counter
		current = persistent["counter"]
		newValue = int(current) + 1
		persistent["counter"] = string(newValue)

		// Emit the new counter value
		emit(newValue)
		return newValue
	}
	`

	tokens, err := marigold.Lex(code)
	if err != nil {
		t.Fatalf("Failed to lex contract: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Failed to parse contract: %v", err)
	}

	ctx := marigold.CodegenWithContext(ast)
	t.Logf("Generated %d instructions", len(ctx.Instructions))

	var bytecodeStr string
	for i, inst := range ctx.Instructions {
		if inst.Opcode == types.PUSH && inst.Value != nil {
			bytecodeStr += "PUSH " + *inst.Value + " "
		} else {
			bytecodeStr += inst.Opcode.String() + " "
		}
		if (i+1)%10 == 0 {
			bytecodeStr += "\n"
		}
	}
	t.Logf("Bytecode:\n%s", bytecodeStr)

	instructions := make([]blockchain.Instruction, len(ctx.Instructions))
	for i, instr := range ctx.Instructions {
		instructions[i] = blockchain.Instruction{
			Opcode: instr.Opcode,
			Value:  instr.Value,
		}
	}

	state := getAccountState(t, nodeAddr, deployerPubKey)

	deployTx := blockchain.Transaction{
		From:         deployerPubKey,
		To:           blockchain.PublicKey{},
		Amount:       0,
		Nonce:        state.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: instructions,
		GasLimit:     100000,
		GasPrice:     100,
		Data:         nil,
	}
	deployTx.Signature = deployTx.GetSignature(priv)

	t.Log("Mining block with contract deployment")
	block2 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&deployTx})
	submitBlock(t, nodeAddr, block2)
	time.Sleep(200 * time.Millisecond)

	time.Sleep(300 * time.Millisecond)

	var contractAddr blockchain.PublicKey
	var contractState *blockchain.AccountState

	allStates, _ := getChainState(t, nodeAddr)
	t.Logf("Total accounts in chain: %d", len(allStates))
	for addr, st := range allStates {
		if len(st.Instructions) > 0 {
			contractAddr = addr
			contractState = st
			t.Logf("Found contract at %x with %d instructions", addr[:8], len(st.Instructions))
			break
		}
	}

	if contractState == nil {
		t.Fatalf("No contract found with instructions")
	}
	t.Logf("Contract has %d instructions", len(contractState.Instructions))

	// Check persistent storage after deployment
	t.Logf("Contract persistent storage after deployment:")
	for k, v := range contractState.Persistent {
		t.Logf("  %s = %s", k, v)
	}

	// Call the contract to increment counter
	t.Log("Calling contract to increment counter")
	updatedDeployerState := getAccountState(t, nodeAddr, deployerPubKey)
	callTx := blockchain.Transaction{
		From:         deployerPubKey,
		To:           contractAddr,
		Amount:       0,
		Nonce:        updatedDeployerState.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: nil,
		GasLimit:     50000,
		GasPrice:     100,
		Data:         nil,
	}
	callTx.Signature = callTx.GetSignature(priv)

	t.Log("Mining block with contract call")
	block3 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&callTx})
	submitBlock(t, nodeAddr, block3)
	time.Sleep(200 * time.Millisecond)

	// Check persistent storage after call
	finalContractState := getAccountState(t, nodeAddr, contractAddr)
	t.Logf("Contract persistent storage after call:")
	for k, v := range finalContractState.Persistent {
		t.Logf("  %s = %s", k, v)
	}

	// Verify the counter was incremented
	if finalContractState.Persistent["counter"] != "1" {
		t.Fatalf("Expected counter to be '1', got '%s'", finalContractState.Persistent["counter"])
	}

	if finalContractState.Persistent["owner"] != "deployer" {
		t.Fatalf("Expected owner to be 'deployer', got '%s'", finalContractState.Persistent["owner"])
	}

	t.Logf("Test passed: Persistent storage working correctly")
}

// Helper functions (copied from deploy_integration_test.go)
func mineBlock(t *testing.T, nodeAddr string, beneficiary blockchain.PublicKey, txs []*blockchain.Transaction) *blockchain.Block {
	chainInfo, err := getChainInfo(t, nodeAddr)
	if err != nil {
		t.Fatalf("Failed to get chain info: %v", err)
	}

	currentStates, err := getChainState(t, nodeAddr)
	if err != nil {
		t.Fatalf("Failed to get chain state: %v", err)
	}

	timestamp := uint64(time.Now().Unix())
	if chainInfo.Height > 0 {
		lastBlock, err := getLastBlock(t, nodeAddr, chainInfo.Hash)
		if err == nil && timestamp <= lastBlock.Header.Timestamp {
			timestamp = lastBlock.Header.Timestamp + 1
		}
	}

	var totalGasFees uint64
	for _, tx := range txs {
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
		To:           beneficiary,
		Amount:       coinbaseAmount,
		Nonce:        0,
		Timestamp:    timestamp,
		Signature:    blockchain.Signature{},
		ChainID:      config.MainnetChainID,
		Instructions: nil,
		GasLimit:     0,
		GasPrice:     0,
	}

	txList := make([]blockchain.Transaction, len(txs))
	for i, tx := range txs {
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
		t.Fatalf("Failed to mine block: %v", err)
	}

	return &block
}

func submitBlock(t *testing.T, nodeAddr string, block *blockchain.Block) {
	blockData, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal block: %v", err)
	}

	url := fmt.Sprintf("http://%s/submit-block", nodeAddr)
	resp, err := http.Post(url, "application/json", bytes.NewReader(blockData))
	if err != nil {
		t.Fatalf("Failed to submit block: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP error: %s", resp.Status)
	}

	time.Sleep(200 * time.Millisecond)
}

func getBalance(t *testing.T, nodeAddr string, pubKey blockchain.PublicKey) uint64 {
	addrHex := hex.EncodeToString(pubKey[:])
	url := fmt.Sprintf("http://%s/balance/%s", nodeAddr, addrHex)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP error: %s", resp.Status)
	}

	var balanceResp node.BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		t.Fatalf("Failed to decode balance response: %v", err)
	}

	return balanceResp.Balance
}

func getAccountState(t *testing.T, nodeAddr string, pubKey blockchain.PublicKey) *blockchain.AccountState {
	states, err := getChainState(t, nodeAddr)
	if err != nil {
		t.Fatalf("Failed to get chain state: %v", err)
	}

	state, exists := states[pubKey]
	if !exists {
		return &blockchain.AccountState{
			Address:      pubKey,
			Balance:      0,
			Nonce:        0,
			Instructions: nil,
			Persistent:   make(map[string]string),
		}
	}

	return state
}

func getChainInfo(t *testing.T, nodeAddr string) (*blockchain.ChainHead, error) {
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

func getChainState(t *testing.T, nodeAddr string) (map[blockchain.PublicKey]*blockchain.AccountState, error) {
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

func getLastBlock(t *testing.T, nodeAddr string, blockHash blockchain.Hash32) (*blockchain.Block, error) {
	hashHex := hex.EncodeToString(blockHash[:])
	url := fmt.Sprintf("http://%s/block/%s", nodeAddr, hashHex)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	var blockResp node.BlockResponse
	if err := json.NewDecoder(resp.Body).Decode(&blockResp); err != nil {
		return nil, fmt.Errorf("failed to decode block response: %w", err)
	}

	if !blockResp.Found {
		return nil, fmt.Errorf("block not found")
	}

	blockData, err := json.Marshal(blockResp.Block)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block data: %w", err)
	}

	var block blockchain.Block
	if err := json.Unmarshal(blockData, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}