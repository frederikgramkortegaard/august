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

func TestDeployAndRunSimpleContract(t *testing.T) {
	// Create a test blockchain starting with genesis
	chain := &blockchain.Chain{
		Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	chain.InitializeIndexes()

	// Create initial account state from genesis
	chain.AccountStates[config.FirstUser] = &blockchain.AccountState{
		Address: config.FirstUser,
		Balance: 10 * config.AUG, // From genesis
		Nonce:   0,
	}

	// Generate a miner keypair
	minerPub, minerPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate miner keys: %v", err)
	}
	var minerPublicKey blockchain.PublicKey
	copy(minerPublicKey[:], minerPub)

	t.Logf("Miner address: %x", minerPublicKey[:8])

	// Mine block 1 to give miner some coins
	coinbaseTx1 := blockchain.Transaction{
		From:             blockchain.PublicKey{}, // Coinbase
		To:               minerPublicKey,
		Amount:           config.BlockReward,
		Nonce:            0,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          config.MainnetChainID,
		Instructions:     nil,
		InitInstructions: nil,
		GasLimit:         0,
		GasPrice:         0,
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

	t.Logf("Mined block 1, miner now has %d AUG", chain.AccountStates[minerPublicKey].Balance/config.AUG)

	// Create a simple contract with a counter
	// Init: Push 0 onto stack and exit (sets initial state)
	initInstructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(0)), // Push initial counter value (0)
		avm.MakeInstruction(avm.EMIT),                         // Emit the result
		// No STOP - let it complete naturally
	}

	// Runtime: Push two numbers, add them, emit result
	runtimeInstructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(5)), // Push 5
		avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(3)), // Push 3
		avm.MakeInstruction(avm.ADD),                          // Add them (5+3=8)
		avm.MakeInstruction(avm.EMIT),            // Emit the result
		avm.MakeInstruction(avm.STOP),            // Stop execution
	}

	// Deploy the contract
	deployTx := blockchain.Transaction{
		From:             minerPublicKey,
		To:               blockchain.PublicKey{}, // Empty = contract deployment
		Amount:           1 * config.AUG,     // Send 1 AUG to contract
		Nonce:            1,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          config.MainnetChainID,
		Instructions:     runtimeInstructions,
		InitInstructions: initInstructions,
		GasLimit:         25000, // 25k gas limit (more reasonable)
		GasPrice:         1000,  // 1000 leaf per gas
	}

	deployTx.Signature = deployTx.GetSignature(minerPriv)

	// Calculate actual gas for coinbase (deployment + init)
	// Base deployment + PUSH (101) + EMIT (2) = 21000 + 103
	deploymentGas := config.GasContractDeploy + 103 // Base deployment cost + init execution
	deploymentGasFees := deploymentGas * deployTx.GasPrice
	coinbaseAmount2 := config.BlockReward + deploymentGasFees

	coinbaseTx2 := blockchain.Transaction{
		From:             blockchain.PublicKey{},
		To:               minerPublicKey,
		Amount:           coinbaseAmount2,
		Nonce:            0,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          config.MainnetChainID,
		Instructions:     nil,
		InitInstructions: nil,
		GasLimit:         0,
		GasPrice:         0,
	}

	// Mine block 2 with contract deployment
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

	t.Logf("Deployed contract in block 2")

	// Find the deployed contract
	var contractAddr blockchain.PublicKey
	var contractFound bool

	for addr, state := range chain.AccountStates {
		if len(state.Instructions) > 0 {
			contractAddr = addr
			contractFound = true
			t.Logf("Found contract at: %x", addr[:8])
			break
		}
	}

	if !contractFound {
		t.Fatalf("Contract was not deployed")
	}

	// Now create a transaction to CALL the contract
	callTx := blockchain.Transaction{
		From:             minerPublicKey,
		To:               contractAddr, // Call the deployed contract
		Amount:           0,            // Don't send any AUG
		Nonce:            2,            // Miner's second transaction
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          config.MainnetChainID,
		Instructions:     nil, // No new code - we're calling existing contract
		InitInstructions: nil,
		GasLimit:         25000, // 25k gas limit for execution
		GasPrice:         1000,  // 1000 leaf per gas
	}

	callTx.Signature = callTx.GetSignature(minerPriv)

	// Execute the transaction to get actual gas usage
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

	actualCallGas, err := blockchain.ApplyTransaction(&callTx, tempStates)
	if err != nil {
		t.Fatalf("Failed to execute call transaction: %v", err)
	}
	t.Logf("Contract call used %d gas", actualCallGas)

	callGasFees := actualCallGas * callTx.GasPrice
	coinbaseAmount3 := config.BlockReward + callGasFees

	coinbaseTx3 := blockchain.Transaction{
		From:             blockchain.PublicKey{},
		To:               minerPublicKey,
		Amount:           coinbaseAmount3,
		Nonce:            0,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          config.MainnetChainID,
		Instructions:     nil,
		InitInstructions: nil,
		GasLimit:         0,
		GasPrice:         0,
	}

	// Mine block 3 with contract call
	previousHash3 := block2.Header.GetHash()
	blockParams3 := blockchain.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash3,
		Height:        3,
		PreviousWork:  block2.Header.TotalWork,
		Coinbase:      coinbaseTx3,
		Transactions:  []blockchain.Transaction{callTx},
		Timestamp:     block2.Header.Timestamp + 1,
		TargetBits:    config.TestTargetCompact,
		CurrentStates: chain.AccountStates,
	}

	t.Logf("Attempting to call contract...")
	t.Logf("Chain has %d accounts before mining block 3", len(chain.AccountStates))
	for addr, state := range chain.AccountStates {
		if len(state.Instructions) > 0 {
			t.Logf("Found contract at %x with %d instructions", addr[:8], len(state.Instructions))
		}
	}

	block3, err := blockchain.NewBlock(blockParams3)
	if err != nil {
		t.Fatalf("Failed to mine block 3: %v", err)
	}

	err = blockchain.ValidateAndApplyBlock(&block3, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 3: %v", err)
	}

	t.Logf("Called contract in block 3")

	// Verify the contract call was executed
	finalMinerState := chain.AccountStates[minerPublicKey]
	t.Logf("Final miner balance: %d AUG", finalMinerState.Balance/config.AUG)
	t.Logf("Final miner nonce: %d", finalMinerState.Nonce)

	// Verify contract state (should still exist)
	contractState := chain.AccountStates[contractAddr]
	if contractState == nil {
		t.Errorf("Contract disappeared after call")
	} else {
		t.Logf("Contract balance after call: %d AUG", contractState.Balance/config.AUG)
	}

	if finalMinerState.Nonce != 2 {
		t.Errorf("Miner nonce should be 2 after two transactions, got %d", finalMinerState.Nonce)
	}

	t.Logf("Contract deployment and execution test completed")
	t.Logf("Blockchain has %d blocks", len(chain.Blocks))
}
