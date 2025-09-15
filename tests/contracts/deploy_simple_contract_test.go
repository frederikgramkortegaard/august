package contracts

import (
	"august/avm"
	"august/blockchain"
	"august/miner"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestDeploySimpleContract(t *testing.T) {
	// Create a test blockchain starting with genesis
	chain := &blockchain.Chain{
		Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	chain.InitializeIndexes()

	// Create initial account state from genesis
	chain.AccountStates[blockchain.FirstUser] = &blockchain.AccountState{
		Address: blockchain.FirstUser,
		Balance: 10 * blockchain.AUG, // From genesis
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
	t.Logf("Genesis user balance: %d AUG", chain.AccountStates[blockchain.FirstUser].Balance/blockchain.AUG)

	// Mine a block to give the miner some coins
	coinbaseTx := blockchain.Transaction{
		From:             blockchain.PublicKey{}, // Coinbase
		To:               minerPublicKey,
		Amount:           blockchain.BlockReward,
		Nonce:            0,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          blockchain.MainnetChainID,
		Instructions:     nil,
		InitInstructions: nil,
		GasLimit:         0,
		GasPrice:         0,
	}

	// Create and mine block 1
	previousHash := blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header)
	blockParams := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash,
		Height:        1,
		PreviousWork:  blockchain.GenesisBlock.Header.TotalWork,
		Coinbase:      coinbaseTx,
		Transactions:  []blockchain.Transaction{}, // No regular transactions yet
		Timestamp:     uint64(time.Now().Unix()),
		TargetBits:    blockchain.TestTargetCompact,
		CurrentStates: chain.AccountStates, // Pass current blockchain state
	}

	block1, err := miner.NewBlock(blockParams)
	if err != nil {
		t.Fatalf("Failed to mine block 1: %v", err)
	}

	// Apply block 1 to the chain
	err = blockchain.ValidateAndApplyBlock(&block1, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 1: %v", err)
	}

	t.Logf("Mined block 1, miner now has %d AUG", chain.AccountStates[minerPublicKey].Balance/blockchain.AUG)

	// Create simple contract with just NOOP instructions
	initInstructions := []avm.Instruction{
		{Opcode: avm.NOOP}, // Do nothing in init - this will succeed
	}

	runtimeInstructions := []avm.Instruction{
		{Opcode: avm.NOOP}, // Do nothing in runtime
		{Opcode: avm.STOP}, // Stop execution
	}

	// Now create a contract deployment transaction from the miner
	deployTx := blockchain.Transaction{
		From:             minerPublicKey,
		To:               blockchain.PublicKey{}, // Empty To address = contract deployment
		Amount:           1 * blockchain.AUG,     // Send 1 AUG to the contract
		Nonce:            1,                      // Miner's first transaction
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          blockchain.MainnetChainID,
		Instructions:     runtimeInstructions,
		InitInstructions: initInstructions,
		GasLimit:         25000, // 25k gas limit
		GasPrice:         1000,  // 1000 leaf per gas
	}

	// Sign the transaction with miner's private key
	blockchain.SignTransaction(&deployTx, minerPriv)

	t.Logf("Creating contract deployment transaction...")
	t.Logf("Miner balance before deployment: %d AUG", chain.AccountStates[minerPublicKey].Balance/blockchain.AUG)

	// Calculate expected gas fees (actual execution: base deployment + init gas)
	actualGas := blockchain.GasContractDeploy + 1 // Base deployment + 1 NOOP instruction
	actualGasFees := actualGas * deployTx.GasPrice
	expectedCoinbaseAmount := blockchain.BlockReward + actualGasFees

	// Create a coinbase transaction for the next block
	coinbaseTx2 := blockchain.Transaction{
		From:             blockchain.PublicKey{}, // Coinbase
		To:               minerPublicKey,
		Amount:           expectedCoinbaseAmount,
		Nonce:            0,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          blockchain.MainnetChainID,
		Instructions:     nil,
		InitInstructions: nil,
		GasLimit:         0,
		GasPrice:         0,
	}

	t.Logf("Expected coinbase amount: %d (block reward %d + gas fees %d)",
		expectedCoinbaseAmount, blockchain.BlockReward, actualGasFees)

	// Mine block 2 with the contract deployment transaction
	previousHash2 := blockchain.HashBlockHeader(&block1.Header)
	blockParams2 := miner.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash2,
		Height:        2,
		PreviousWork:  block1.Header.TotalWork,
		Coinbase:      coinbaseTx2,
		Transactions:  []blockchain.Transaction{deployTx}, // Include deployment transaction
		Timestamp:     block1.Header.Timestamp + 1,        // Ensure timestamp is after block 1
		TargetBits:    blockchain.TestTargetCompact,
		CurrentStates: chain.AccountStates, // Pass current blockchain state
	}

	block2, err := miner.NewBlock(blockParams2)
	if err != nil {
		t.Fatalf("Failed to mine block 2: %v", err)
	}

	// Apply block 2 to the chain
	err = blockchain.ValidateAndApplyBlock(&block2, chain)
	if err != nil {
		t.Fatalf("Failed to apply block 2: %v", err)
	}

	t.Logf("Mined block 2 with contract deployment")

	// Check that miner's balance was deducted correctly
	minerState := chain.AccountStates[minerPublicKey]
	t.Logf("Miner balance after deployment: %d AUG", minerState.Balance/blockchain.AUG)

	// Check that miner's nonce was incremented
	if minerState.Nonce != 1 {
		t.Errorf("Miner nonce not incremented: got %d, expected 1", minerState.Nonce)
	}

	// Find the deployed contract (look for account with contract code)
	var contractAddr blockchain.PublicKey
	var contractState *blockchain.AccountState
	var contractFound bool

	for addr, state := range chain.AccountStates {
		if len(state.Instructions) > 0 { // This is a contract
			contractAddr = addr
			contractState = state
			contractFound = true
			t.Logf("Found deployed contract at: %x", addr[:8])
			break
		}
	}

	if !contractFound {
		t.Fatalf("No contract was deployed")
	}

	// Verify contract state
	if contractState.Balance != deployTx.Amount {
		t.Errorf("Contract balance incorrect: got %d, expected %d", contractState.Balance, deployTx.Amount)
	}

	if contractState.Nonce != 0 {
		t.Errorf("Contract nonce should be 0: got %d", contractState.Nonce)
	}

	if len(contractState.Instructions) != len(runtimeInstructions) {
		t.Errorf("Contract instructions length mismatch: got %d, expected %d", len(contractState.Instructions), len(runtimeInstructions))
	}

	// Verify runtime instructions are stored correctly
	for i, instruction := range contractState.Instructions {
		if instruction.Opcode != runtimeInstructions[i].Opcode {
			t.Errorf("Contract instruction %d opcode mismatch: got %s, expected %s",
				i, instruction.Opcode, runtimeInstructions[i].Opcode)
		}
	}

	// Verify code hash is computed correctly
	expectedCodeHash := blockchain.ComputeCodeHash(runtimeInstructions)
	if contractState.CodeHash != expectedCodeHash {
		t.Errorf("Contract code hash mismatch: got %x, expected %x", contractState.CodeHash, expectedCodeHash)
	}

	t.Logf("Simple NOOP contract deployed successfully at: %x", contractAddr[:8])
	t.Logf("Contract has %d AUG balance", contractState.Balance/blockchain.AUG)
	t.Logf("Blockchain now has %d blocks", len(chain.Blocks))
}

// TODO: Add test for failed contract deployment
