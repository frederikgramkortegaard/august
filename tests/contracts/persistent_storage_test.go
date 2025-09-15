package contracts

import (
	"august/avm"
	"august/blockchain"
	"august/miner"
	"crypto/ed25519"
	"math/big"
	"testing"
	"time"
)

func TestPersistentStorageBasics(t *testing.T) {
	// Create a test blockchain starting with genesis
	chain := &blockchain.Chain{
		Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	chain.InitializeIndexes()

	// Create initial account state from genesis
	chain.AccountStates[blockchain.FirstUser] = &blockchain.AccountState{
		Address: blockchain.FirstUser,
		Balance: 10 * blockchain.AUG,
		Nonce:   0,
	}

	// Generate a miner keypair
	minerPub, minerPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate miner keys: %v", err)
	}
	var minerPublicKey blockchain.PublicKey
	copy(minerPublicKey[:], minerPub)

	// Mine block 1 to give miner some coins
	coinbaseTx1 := blockchain.Transaction{
		From:      blockchain.PublicKey{}, // Coinbase
		To:        minerPublicKey,
		Amount:    blockchain.BlockReward,
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   blockchain.MainnetChainID,
		GasLimit:  0,
		GasPrice:  0,
	}

	previousHash := blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header)
	blockParams1 := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash,
		Height:        1,
		PreviousWork:  blockchain.GenesisBlock.Header.TotalWork,
		Coinbase:      coinbaseTx1,
		Transactions:  []blockchain.Transaction{},
		Timestamp:     uint64(time.Now().Unix()),
		TargetBits:    blockchain.TestTargetCompact,
		CurrentStates: chain.AccountStates,
	}

	block1, err := miner.NewBlock(blockParams1)
	if err != nil {
		t.Fatalf("Failed to mine block 1: %v", err)
	}

	err = blockchain.ValidateAndApplyBlock(&block1, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 1: %v", err)
	}

	t.Logf("Miner now has %d AUG", chain.AccountStates[minerPublicKey].Balance/blockchain.AUG)

	// Create a contract that uses persistent storage
	// Init: Store initial value 42 at address 1
	initInstructions := []avm.Instruction{
		{Opcode: avm.PUSH, Value: big.NewInt(42)}, // Push value 42
		{Opcode: avm.PUSH, Value: big.NewInt(1)},  // Push address 1
		{Opcode: avm.PSTORE},                      // Store 42 at address 1
		{Opcode: avm.PUSH, Value: big.NewInt(1)},  // Push address 1
		{Opcode: avm.PLOAD},                       // Load from address 1
		{Opcode: avm.EMIT},                        // Emit the loaded value
	}

	// Runtime: Increment the stored value and store it back
	runtimeInstructions := []avm.Instruction{
		{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Push address 1
		{Opcode: avm.PLOAD},                      // Load current value (stack: [42])
		{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Push increment value (stack: [42, 1])
		{Opcode: avm.ADD},                        // Add 1 to current value (stack: [43])
		{Opcode: avm.DUP},                        // Duplicate result (stack: [43, 43])
		{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Push address 1 (stack: [43, 43, 1])
		{Opcode: avm.SWAP, Param: 1},             // Swap top two (stack: [43, 1, 43])
		{Opcode: avm.POP},                        // Remove extra value (stack: [43, 1])
		{Opcode: avm.PSTORE},                     // Store 43 at address 1 (stack: [])
		{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Push address 1 for loading
		{Opcode: avm.PLOAD},                      // Load to verify (stack: [43])
		{Opcode: avm.EMIT},                       // Emit the new value
		{Opcode: avm.STOP},
	}

	// Deploy the contract
	deployTx := blockchain.Transaction{
		From:             minerPublicKey,
		To:               blockchain.PublicKey{}, // Empty = contract deployment
		Amount:           1 * blockchain.AUG,
		Nonce:            1,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          blockchain.MainnetChainID,
		Instructions:     runtimeInstructions,
		InitInstructions: initInstructions,
		GasLimit:         30000,
		GasPrice:         1000,
	}

	blockchain.SignTransaction(&deployTx, minerPriv)

	// Execute to get actual gas usage for coinbase
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
	t.Logf("Contract deployment used %d gas", actualDeployGas)

	coinbaseTx2 := blockchain.Transaction{
		From:      blockchain.PublicKey{},
		To:        minerPublicKey,
		Amount:    blockchain.BlockReward + (actualDeployGas * deployTx.GasPrice),
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   blockchain.MainnetChainID,
		GasLimit:  0,
		GasPrice:  0,
	}

	// Mine block 2 with contract deployment
	previousHash2 := blockchain.HashBlockHeader(&block1.Header)
	blockParams2 := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash2,
		Height:        2,
		PreviousWork:  block1.Header.TotalWork,
		Coinbase:      coinbaseTx2,
		Transactions:  []blockchain.Transaction{deployTx},
		Timestamp:     block1.Header.Timestamp + 1,
		TargetBits:    blockchain.TestTargetCompact,
		CurrentStates: chain.AccountStates,
	}

	block2, err := miner.NewBlock(blockParams2)
	if err != nil {
		t.Fatalf("Failed to mine block 2: %v", err)
	}

	err = blockchain.ValidateAndApplyBlock(&block2, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 2: %v", err)
	}

	t.Logf("Contract deployed in block 2")

	// Find the deployed contract
	var contractAddr blockchain.PublicKey
	var contractState *blockchain.AccountState
	var contractFound bool

	for addr, state := range chain.AccountStates {
		if len(state.Instructions) > 0 {
			contractAddr = addr
			contractState = state
			contractFound = true
			t.Logf("Found contract at: %x", addr[:8])
			break
		}
	}

	if !contractFound {
		t.Fatalf("Contract was not deployed")
	}

	// Verify initial persistent storage from init execution
	if len(contractState.Persistent) == 0 {
		t.Errorf("Contract should have persistent storage after init")
	} else {
		value, exists := contractState.Persistent["1"] // Address 1
		if !exists {
			t.Errorf("Expected value at address 1 in persistent storage")
		} else if value != "42" {
			t.Errorf("Expected initial value 42, got %s", value)
		} else {
			t.Logf("Initial persistent storage: address 1 = %s", value)
		}
	}

	// Now call the contract to increment the stored value
	callTx := blockchain.Transaction{
		From:             minerPublicKey,
		To:               contractAddr,
		Amount:           0,
		Nonce:            2,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          blockchain.MainnetChainID,
		Instructions:     nil,
		InitInstructions: nil,
		GasLimit:         30000,
		GasPrice:         1000,
	}

	blockchain.SignTransaction(&callTx, minerPriv)

	// Execute to get actual gas usage
	tempStates2 := make(map[blockchain.PublicKey]*blockchain.AccountState)
	for pubKey, state := range chain.AccountStates {
		if state != nil {
			tempStates2[pubKey] = &blockchain.AccountState{
				Address:      state.Address,
				Balance:      state.Balance,
				Nonce:        state.Nonce,
				Instructions: state.Instructions,
				Persistent:   make(map[string]string),
				StorageRoot:  state.StorageRoot,
				CodeHash:     state.CodeHash,
			}
			for k, v := range state.Persistent {
				tempStates2[pubKey].Persistent[k] = v
			}
		}
	}

	actualCallGas, err := blockchain.ApplyTransaction(&callTx, tempStates2)
	if err != nil {
		t.Fatalf("Failed to execute call transaction: %v", err)
	}
	t.Logf("Contract call used %d gas", actualCallGas)

	coinbaseTx3 := blockchain.Transaction{
		From:      blockchain.PublicKey{},
		To:        minerPublicKey,
		Amount:    blockchain.BlockReward + (actualCallGas * callTx.GasPrice),
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   blockchain.MainnetChainID,
		GasLimit:  0,
		GasPrice:  0,
	}

	// Mine block 3 with contract call
	previousHash3 := blockchain.HashBlockHeader(&block2.Header)
	blockParams3 := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash3,
		Height:        3,
		PreviousWork:  block2.Header.TotalWork,
		Coinbase:      coinbaseTx3,
		Transactions:  []blockchain.Transaction{callTx},
		Timestamp:     block2.Header.Timestamp + 1,
		TargetBits:    blockchain.TestTargetCompact,
		CurrentStates: chain.AccountStates,
	}

	block3, err := miner.NewBlock(blockParams3)
	if err != nil {
		t.Fatalf("Failed to mine block 3: %v", err)
	}

	err = blockchain.ValidateAndApplyBlock(&block3, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 3: %v", err)
	}

	t.Logf("Contract called in block 3")

	// Verify persistent storage was updated
	finalContractState := chain.AccountStates[contractAddr]
	if len(finalContractState.Persistent) == 0 {
		t.Errorf("Contract should still have persistent storage after call")
	} else {
		value, exists := finalContractState.Persistent["1"] // Address 1
		if !exists {
			t.Errorf("Expected value at address 1 in persistent storage after call")
		} else if value != "43" { // 42 + 1 = 43
			t.Errorf("Expected incremented value 43, got %s", value)
		} else {
			t.Logf("Updated persistent storage: address 1 = %s", value)
		}
	}

	t.Logf("Persistent storage test completed successfully!")
	t.Logf("Initial value: 42 -> Final value: %s", finalContractState.Persistent["1"])
}

func TestMultiplePersistentStorageSlots(t *testing.T) {
	// Test using multiple storage slots
	// This test will create a contract that stores different values at different addresses

	// Create test blockchain
	chain := &blockchain.Chain{
		Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	chain.InitializeIndexes()

	chain.AccountStates[blockchain.FirstUser] = &blockchain.AccountState{
		Address: blockchain.FirstUser,
		Balance: 10 * blockchain.AUG,
		Nonce:   0,
	}

	// Generate miner keypair
	minerPub, minerPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate miner keys: %v", err)
	}
	var minerPublicKey blockchain.PublicKey
	copy(minerPublicKey[:], minerPub)

	// Mine block 1 for initial funding
	coinbaseTx1 := blockchain.Transaction{
		From:      blockchain.PublicKey{},
		To:        minerPublicKey,
		Amount:    blockchain.BlockReward,
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   blockchain.MainnetChainID,
		GasLimit:  0,
		GasPrice:  0,
	}

	previousHash := blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header)
	blockParams1 := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash,
		Height:        1,
		PreviousWork:  blockchain.GenesisBlock.Header.TotalWork,
		Coinbase:      coinbaseTx1,
		Transactions:  []blockchain.Transaction{},
		Timestamp:     uint64(time.Now().Unix()),
		TargetBits:    blockchain.TestTargetCompact,
		CurrentStates: chain.AccountStates,
	}

	block1, err := miner.NewBlock(blockParams1)
	if err != nil {
		t.Fatalf("Failed to mine block 1: %v", err)
	}

	err = blockchain.ValidateAndApplyBlock(&block1, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 1: %v", err)
	}

	// Init: Store multiple values at different addresses
	initInstructions := []avm.Instruction{
		// Store 100 at address 1
		{Opcode: avm.PUSH, Value: big.NewInt(100)},
		{Opcode: avm.PUSH, Value: big.NewInt(1)},
		{Opcode: avm.PSTORE},
		// Store 200 at address 2
		{Opcode: avm.PUSH, Value: big.NewInt(200)},
		{Opcode: avm.PUSH, Value: big.NewInt(2)},
		{Opcode: avm.PSTORE},
		// Store 300 at address 3
		{Opcode: avm.PUSH, Value: big.NewInt(300)},
		{Opcode: avm.PUSH, Value: big.NewInt(3)},
		{Opcode: avm.PSTORE},
	}

	// Runtime: Load all values, add them up, and emit result
	runtimeInstructions := []avm.Instruction{
		{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Load from address 1
		{Opcode: avm.PLOAD},
		{Opcode: avm.PUSH, Value: big.NewInt(2)}, // Load from address 2
		{Opcode: avm.PLOAD},
		{Opcode: avm.ADD},                        // Add first two values
		{Opcode: avm.PUSH, Value: big.NewInt(3)}, // Load from address 3
		{Opcode: avm.PLOAD},
		{Opcode: avm.ADD},  // Add third value (100+200+300=600)
		{Opcode: avm.EMIT}, // Emit the sum
		{Opcode: avm.STOP},
	}

	// Deploy contract
	deployTx := blockchain.Transaction{
		From:             minerPublicKey,
		To:               blockchain.PublicKey{},
		Amount:           1 * blockchain.AUG,
		Nonce:            1,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          blockchain.MainnetChainID,
		Instructions:     runtimeInstructions,
		InitInstructions: initInstructions,
		GasLimit:         30000,
		GasPrice:         1000,
	}

	blockchain.SignTransaction(&deployTx, minerPriv)

	// Get gas usage and deploy
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
		Amount:    blockchain.BlockReward + (actualDeployGas * deployTx.GasPrice),
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   blockchain.MainnetChainID,
		GasLimit:  0,
		GasPrice:  0,
	}

	// Mine deployment block
	previousHash2 := blockchain.HashBlockHeader(&block1.Header)
	blockParams2 := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash2,
		Height:        2,
		PreviousWork:  block1.Header.TotalWork,
		Coinbase:      coinbaseTx2,
		Transactions:  []blockchain.Transaction{deployTx},
		Timestamp:     block1.Header.Timestamp + 1,
		TargetBits:    blockchain.TestTargetCompact,
		CurrentStates: chain.AccountStates,
	}

	block2, err := miner.NewBlock(blockParams2)
	if err != nil {
		t.Fatalf("Failed to mine block 2: %v", err)
	}

	err = blockchain.ValidateAndApplyBlock(&block2, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 2: %v", err)
	}

	// Find contract and verify multiple storage slots
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

	// Verify all three storage slots
	expectedValues := map[string]string{
		"1": "100",
		"2": "200",
		"3": "300",
	}

	for addr, expectedValue := range expectedValues {
		actualValue, exists := contractState.Persistent[addr]
		if !exists {
			t.Errorf("Expected value at address %s", addr)
		} else if actualValue != expectedValue {
			t.Errorf("At address %s: expected %s, got %s", addr, expectedValue, actualValue)
		} else {
			t.Logf("Storage slot %s = %s", addr, actualValue)
		}
	}

	// Now call the contract to sum the values
	callTx := blockchain.Transaction{
		From:      minerPublicKey,
		To:        contractAddr,
		Amount:    0,
		Nonce:     2,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   blockchain.MainnetChainID,
		GasLimit:  30000,
		GasPrice:  1000,
	}

	blockchain.SignTransaction(&callTx, minerPriv)

	tempStates2 := make(map[blockchain.PublicKey]*blockchain.AccountState)
	for pubKey, state := range chain.AccountStates {
		if state != nil {
			tempStates2[pubKey] = &blockchain.AccountState{
				Address:      state.Address,
				Balance:      state.Balance,
				Nonce:        state.Nonce,
				Instructions: state.Instructions,
				Persistent:   make(map[string]string),
				StorageRoot:  state.StorageRoot,
				CodeHash:     state.CodeHash,
			}
			for k, v := range state.Persistent {
				tempStates2[pubKey].Persistent[k] = v
			}
		}
	}

	actualCallGas, err := blockchain.ApplyTransaction(&callTx, tempStates2)
	if err != nil {
		t.Fatalf("Failed to execute call transaction: %v", err)
	}

	coinbaseTx3 := blockchain.Transaction{
		From:      blockchain.PublicKey{},
		To:        minerPublicKey,
		Amount:    blockchain.BlockReward + (actualCallGas * callTx.GasPrice),
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   blockchain.MainnetChainID,
		GasLimit:  0,
		GasPrice:  0,
	}

	// Mine call block
	previousHash3 := blockchain.HashBlockHeader(&block2.Header)
	blockParams3 := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash3,
		Height:        3,
		PreviousWork:  block2.Header.TotalWork,
		Coinbase:      coinbaseTx3,
		Transactions:  []blockchain.Transaction{callTx},
		Timestamp:     block2.Header.Timestamp + 1,
		TargetBits:    blockchain.TestTargetCompact,
		CurrentStates: chain.AccountStates,
	}

	block3, err := miner.NewBlock(blockParams3)
	if err != nil {
		t.Fatalf("Failed to mine block 3: %v", err)
	}

	err = blockchain.ValidateAndApplyBlock(&block3, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 3: %v", err)
	}

	t.Logf("Multiple storage slots test completed - contract should have emitted 600 (100+200+300)")
}
