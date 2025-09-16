package contracts

import (
	"august/avm"
	"august/blockchain"
	"august/config"
		"crypto/ed25519"
	"math/big"
	"testing"
	"time"
)

func TestSimpleStorageDebug(t *testing.T) {
	// Simplest possible test: just store and load a value during init

	chain := &blockchain.Chain{
		Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	chain.InitializeIndexes()

	chain.AccountStates[config.FirstUser] = &blockchain.AccountState{
		Address: config.FirstUser,
		Balance: 10 * config.AUG,
		Nonce:   0,
	}

	// Generate miner keypair
	minerPub, minerPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate miner keys: %v", err)
	}
	var minerPublicKey blockchain.PublicKey
	copy(minerPublicKey[:], minerPub)

	// Mine block 1 for funding
	coinbaseTx1 := blockchain.Transaction{
		From:      blockchain.PublicKey{},
		To:        minerPublicKey,
		Amount:    config.BlockReward,
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   config.MainnetChainID,
		GasLimit:  0,
		GasPrice:  0,
	}

	previousHash := blockchain.GenesisBlock.Header.GetHash()
	blockParams1 := blockchain.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash,
		Height:        1,
		PreviousWork:  blockchain.GenesisBlock.Header.TotalWork,
		Coinbase:      coinbaseTx1,
		Transactions:  []blockchain.Transaction{},
		Timestamp:     uint64(time.Now().Unix()),
		TargetBits:    config.TestTargetCompact,
		CurrentStates: chain.AccountStates,
	}

	block1, err := blockchain.NewBlock(blockParams1)
	if err != nil {
		t.Fatalf("Failed to mine block 1: %v", err)
	}

	err = blockchain.ValidateAndApplyBlock(&block1, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 1: %v", err)
	}

	// Create simple init that just stores and emits a value
	initInstructions := []avm.Instruction{
		{Opcode: avm.PUSH, Value: big.NewInt(123)}, // Push 123
		{Opcode: avm.PUSH, Value: big.NewInt(5)},   // Push address 5
		{Opcode: avm.PSTORE},                       // Store 123 at address 5
		{Opcode: avm.PUSH, Value: big.NewInt(5)},   // Push address 5
		{Opcode: avm.PLOAD},                        // Load from address 5 (should be 123)
		{Opcode: avm.EMIT},                         // Emit it (should show 123)
	}

	// Runtime is just STOP
	runtimeInstructions := []avm.Instruction{
		{Opcode: avm.STOP},
	}

	// Deploy contract
	deployTx := blockchain.Transaction{
		From:             minerPublicKey,
		To:               blockchain.PublicKey{},
		Amount:           1 * config.AUG,
		Nonce:            1,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          config.MainnetChainID,
		Instructions:     runtimeInstructions,
		InitInstructions: initInstructions,
		GasLimit:         25000,
		GasPrice:         1000,
	}

	deployTx.Signature = deployTx.GetSignature(minerPriv)

	// Execute to get gas
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

	actualDeployGas, err := blockchain.ApplyTransaction(&deployTx, tempStates)
	if err != nil {
		t.Fatalf("Failed to execute deploy transaction: %v", err)
	}

	coinbaseTx2 := blockchain.Transaction{
		From:      blockchain.PublicKey{},
		To:        minerPublicKey,
		Amount:    config.BlockReward + (actualDeployGas * deployTx.GasPrice),
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   config.MainnetChainID,
		GasLimit:  0,
		GasPrice:  0,
	}

	// Deploy the contract
	previousHash2 := block1.Header.GetHash()
	blockParams2 := blockchain.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash2,
		Height:        2,
		PreviousWork:  block1.Header.TotalWork,
		Coinbase:      coinbaseTx2,
		Transactions:  []blockchain.Transaction{deployTx},
		Timestamp:     block1.Header.Timestamp + 1,
		TargetBits:    config.TestTargetCompact,
		CurrentStates: chain.AccountStates,
	}

	block2, err := blockchain.NewBlock(blockParams2)
	if err != nil {
		t.Fatalf("Failed to mine block 2: %v", err)
	}

	err = blockchain.ValidateAndApplyBlock(&block2, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 2: %v", err)
	}

	// Find contract and check persistent storage
	var contractAddr blockchain.PublicKey
	var contractState *blockchain.AccountState
	var contractFound bool

	for addr, state := range chain.AccountStates {
		if len(state.Instructions) > 0 {
			contractAddr = addr
			contractState = state
			contractFound = true
			break
		}
	}

	if !contractFound {
		t.Fatalf("Contract was not deployed")
	}

	t.Logf("Contract deployed at: %x", contractAddr[:8])
	t.Logf("Contract has %d persistent storage entries", len(contractState.Persistent))

	// Check if value 123 was stored at address 5
	if value, exists := contractState.Persistent["5"]; !exists {
		t.Errorf("Expected value at address 5")
	} else if value != "123" {
		t.Errorf("Expected value 123 at address 5, got %s", value)
	} else {
		t.Logf("Persistent storage working: address 5 = %s", value)
	}
}