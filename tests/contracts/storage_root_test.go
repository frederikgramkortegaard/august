package contracts

import (
	"august/avm"
	"august/blockchain"
	"august/config"
	"math/big"
	"testing"
)

func TestStorageRootUpdates(t *testing.T) {
	// Test: StorageRoot should be correctly computed when persistent storage changes

	t.Log("=== Testing StorageRoot Computation ===")

	// Step 1: Create a simple contract state with persistent storage
	t.Log("Step 1: Creating contract with persistent storage")

	persistent := map[string]string{
		"5": "999",
		"7": "123",
		"10": "456",
	}

	contractState := &blockchain.AccountState{
		Address:      blockchain.PublicKey{1, 2, 3}, // Dummy address
		Balance:      1000,
		Nonce:        0,
		Instructions: []avm.Instruction{avm.MakeInstruction(avm.STOP)}, // Simple contract
		Persistent:   persistent,
		StorageRoot:  blockchain.Hash32{}, // Will be computed
		CodeHash:     blockchain.Hash32{},
	}

	t.Logf("Contract has %d storage entries: %v", len(persistent), persistent)

	// Step 2: Compute StorageRoot using the blockchain function
	t.Log("Step 2: Computing StorageRoot")

	expectedRoot := blockchain.ComputeStorageRoot(persistent)
	contractState.StorageRoot = expectedRoot

	t.Logf("Computed StorageRoot: %x", expectedRoot)

	// Step 3: Verify StorageRoot properties
	t.Log("Step 3: Verifying StorageRoot properties")

	// Property 1: StorageRoot should not be empty when storage exists
	emptyRoot := blockchain.Hash32{}
	if contractState.StorageRoot == emptyRoot {
		t.Fatal("StorageRoot should not be empty when persistent storage exists")
	}
	t.Log("StorageRoot is non-empty for non-empty storage")

	// Property 2: Same storage should produce same root
	sameRoot := blockchain.ComputeStorageRoot(persistent)
	if contractState.StorageRoot != sameRoot {
		t.Fatalf("Same storage should produce same root: expected %x, got %x",
			contractState.StorageRoot, sameRoot)
	}
	t.Log("Same storage produces same StorageRoot")

	// Property 3: Different storage should produce different root
	differentStorage := map[string]string{
		"5": "999",
		"7": "124", // Changed value
		"10": "456",
	}
	differentRoot := blockchain.ComputeStorageRoot(differentStorage)
	if contractState.StorageRoot == differentRoot {
		t.Fatal("Different storage should produce different StorageRoot")
	}
	t.Log("Different storage produces different StorageRoot")

	// Property 4: Empty storage should produce empty root
	emptyStorage := make(map[string]string)
	computedEmptyRoot := blockchain.ComputeStorageRoot(emptyStorage)
	if computedEmptyRoot != emptyRoot {
		t.Fatalf("Empty storage should produce empty root, got %x", computedEmptyRoot)
	}
	t.Log("Empty storage produces empty StorageRoot")

	t.Log("=== All StorageRoot properties verified! ===")
}

func TestStorageRootWithContractExecution(t *testing.T) {
	// Test: StorageRoot should be updated during actual contract execution

	t.Log("=== Testing StorageRoot During Contract Execution ===")

	// Step 1: Create a test chain with funded account
	t.Log("Step 1: Setting up test blockchain")

	chain := &blockchain.Chain{
		Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	chain.InitializeIndexes()

	// Create funded test account
	testAccount := blockchain.PublicKey{0x42}
	chain.AccountStates[testAccount] = &blockchain.AccountState{
		Address: testAccount,
		Balance: 10 * config.AUG,
		Nonce:   0,
	}

	t.Log("Test blockchain initialized with funded account")

	// Step 2: Create contract that modifies storage during initialization
	t.Log("Step 2: Creating contract with storage operations")

	initInstructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(777)), // value
		avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(3)),   // address
		avm.MakeInstruction(avm.PSTORE),                         // store 777 at address 3

		avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(888)), // another value
		avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(8)),   // another address
		avm.MakeInstruction(avm.PSTORE),                         // store 888 at address 8
	}

	// Step 3: Deploy contract and verify StorageRoot is computed
	t.Log("Step 3: Deploying contract")

	deployTx := blockchain.Transaction{
		From:             testAccount,
		To:               blockchain.PublicKey{}, // Empty = contract deployment
		Amount:           1 * config.AUG,
		Nonce:            1,
		Instructions:     []avm.Instruction{avm.MakeInstruction(avm.STOP)}, // Runtime code
		InitInstructions: initInstructions,                      // Init code
		GasLimit:         50000,
		GasPrice:         100,
	}

	// Apply transaction to see the storage changes
	gasUsed, err := blockchain.ApplyTransaction(&deployTx, chain.AccountStates)
	if err != nil {
		t.Fatalf("Contract deployment failed: %v", err)
	}

	// Find the deployed contract address
	var contractAddr blockchain.PublicKey
	for addr, state := range chain.AccountStates {
		if len(state.Instructions) > 0 {
			contractAddr = addr
			break
		}
	}

	// Execute the contract initialization code
	_, err = ExecuteContractAfterDeployment(&deployTx, chain.AccountStates, contractAddr)
	if err != nil {
		t.Fatalf("Failed to execute contract: %v", err)
	}

	t.Logf("Contract deployed successfully, gas used: %d", gasUsed)

	// Step 4: Find the deployed contract and verify its StorageRoot
	t.Log("Step 4: Verifying StorageRoot after deployment")

	var contractState *blockchain.AccountState
	for _, state := range chain.AccountStates {
		if len(state.Instructions) > 0 { // Found the contract
			contractState = state
			break
		}
	}

	if contractState == nil {
		t.Fatal("Contract not found after deployment")
	}

	// Check storage content
	expectedStorage := map[string]string{
		"3": "777",
		"8": "888",
	}

	for addr, expectedValue := range expectedStorage {
		if actualValue, exists := contractState.Persistent[addr]; !exists {
			t.Errorf("Missing storage at address %s", addr)
		} else if actualValue != expectedValue {
			t.Errorf("Wrong storage value at address %s: expected %s, got %s",
				addr, expectedValue, actualValue)
		}
	}

	t.Logf("Contract persistent storage: %v", contractState.Persistent)

	// Verify StorageRoot is correctly computed
	expectedRoot := blockchain.ComputeStorageRoot(contractState.Persistent)
	if contractState.StorageRoot != expectedRoot {
		t.Fatalf("StorageRoot mismatch: expected %x, got %x",
			expectedRoot, contractState.StorageRoot)
	}

	// Verify StorageRoot is not empty
	emptyRoot := blockchain.Hash32{}
	if contractState.StorageRoot == emptyRoot {
		t.Fatal("StorageRoot should not be empty when contract has storage")
	}

	t.Logf("StorageRoot correctly computed: %x", contractState.StorageRoot)
	t.Log("=== Contract StorageRoot validation passed! ===")
}