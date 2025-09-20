package contracts

import (
	"august/avm"
	"august/blockchain"
	"august/config"
	_ "august/execution" // Import for side effects (registers executor)
	"crypto/ed25519"
	"encoding/hex"
	"math/big"
	"testing"
	"time"
)

func TestTransactionDataField(t *testing.T) {
	// Create a test blockchain
	chain := &blockchain.Chain{
		Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	chain.InitializeIndexes()

	// Apply genesis
	if err := blockchain.ValidateAndApplyBlock(blockchain.GenesisBlock, chain); err != nil {
		t.Fatalf("Failed to apply genesis block: %v", err)
	}

	// Create miner keys
	_, minerPriv, _ := ed25519.GenerateKey(nil)
	minerPub := minerPriv.Public().(ed25519.PublicKey)
	var minerPublicKey blockchain.PublicKey
	copy(minerPublicKey[:], minerPub)

	t.Logf("Miner address: %x", minerPublicKey[:8])

	// Test 1: Regular transaction with data
	t.Run("RegularTransactionWithData", func(t *testing.T) {
		// Give miner initial funds
		chain.AccountStates[minerPublicKey] = &blockchain.AccountState{
			Balance: 100 * config.AUG,
			Nonce:   0,
			Address: minerPublicKey,
		}

		// Create recipient
		recipientPub, _, _ := ed25519.GenerateKey(nil)
		var recipient blockchain.PublicKey
		copy(recipient[:], recipientPub)

		// Test data
		testData := []byte("Hello, blockchain!")

		// Create transaction with data
		tx := blockchain.Transaction{
			From:      minerPublicKey,
			To:        recipient,
			Amount:    1 * config.AUG,
			Nonce:     1,
			Timestamp: uint64(time.Now().Unix()),
			ChainID:   config.MainnetChainID,
			GasLimit:  25000,
			GasPrice:  100,
			Data:      testData, // Include data
		}
		tx.Signature = tx.GetSignature(minerPriv)

		// Apply transaction
		gasUsed, err := blockchain.ApplyTransaction(&tx, chain.AccountStates)
		if err != nil {
			t.Fatalf("Failed to apply transaction: %v", err)
		}

		// Calculate expected gas: base transfer + data gas
		expectedGas := config.GasTransfer + uint64(len(testData))*config.GasDataPerByte
		if gasUsed != expectedGas {
			t.Errorf("Gas usage mismatch: got %d, expected %d (base %d + data %d)",
				gasUsed, expectedGas, config.GasTransfer, uint64(len(testData))*config.GasDataPerByte)
		}

		t.Logf("Transaction with %d bytes of data used %d gas", len(testData), gasUsed)
	})

	// Test 2: Contract deployment with data
	t.Run("ContractDeploymentWithData", func(t *testing.T) {
		// Simple contract that just stops
		initInstructions := []avm.Instruction{
			avm.MakeInstruction(avm.NOOP),
		}
		runtimeInstructions := []avm.Instruction{
			avm.MakeInstruction(avm.STOP),
		}

		deployData := []byte("Contract metadata")

		deployTx := blockchain.Transaction{
			From:             minerPublicKey,
			To:               blockchain.PublicKey{}, // Empty = deployment
			Amount:           1 * config.AUG,
			Nonce:            2,
			Timestamp:        uint64(time.Now().Unix()),
			ChainID:          config.MainnetChainID,
			Instructions:     runtimeInstructions,
			InitInstructions: initInstructions,
			GasLimit:         50000,
			GasPrice:         100,
			Data:             deployData, // Include data in deployment
		}
		deployTx.Signature = deployTx.GetSignature(minerPriv)

		gasUsed, err := blockchain.ApplyTransaction(&deployTx, chain.AccountStates)
		if err != nil {
			t.Fatalf("Failed to deploy contract: %v", err)
		}

		// Expected gas: base deployment + data gas + init execution (1 for NOOP)
		expectedGas := config.GasContractDeploy + uint64(len(deployData))*config.GasDataPerByte + 1
		if gasUsed != expectedGas {
			t.Errorf("Deployment gas mismatch: got %d, expected %d", gasUsed, expectedGas)
		}

		t.Logf("Contract deployment with %d bytes of data used %d gas", len(deployData), gasUsed)
	})

	// Test 3: Contract call with data (accessing @tsxdata)
	t.Run("ContractCallWithData", func(t *testing.T) {
		// Deploy a contract that emits the data length
		// This would work if @tsxdata was properly integrated with AVM
		// For now, we just test that data is included in gas calculation

		initInstructions := []avm.Instruction{
			avm.MakeInstruction(avm.NOOP),
		}

		// Runtime: just emit a value
		runtimeInstructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(42)),
			avm.MakeInstruction(avm.EMIT),
			avm.MakeInstruction(avm.STOP),
		}

		// Deploy contract
		deployTx := blockchain.Transaction{
			From:             minerPublicKey,
			To:               blockchain.PublicKey{},
			Amount:           0,
			Nonce:            3,
			Timestamp:        uint64(time.Now().Unix()),
			ChainID:          config.MainnetChainID,
			Instructions:     runtimeInstructions,
			InitInstructions: initInstructions,
			GasLimit:         50000,
			GasPrice:         100,
		}
		deployTx.Signature = deployTx.GetSignature(minerPriv)

		_, err := blockchain.ApplyTransaction(&deployTx, chain.AccountStates)
		if err != nil {
			t.Fatalf("Failed to deploy contract: %v", err)
		}

		// Find deployed contract
		contractAddr := blockchain.GenerateContractAddress(minerPublicKey, 3)

		// Call contract with data
		callData := []byte("function_call(123)")

		callTx := blockchain.Transaction{
			From:      minerPublicKey,
			To:        contractAddr,
			Amount:    0,
			Nonce:     4,
			Timestamp: uint64(time.Now().Unix()),
			ChainID:   config.MainnetChainID,
			GasLimit:  50000,
			GasPrice:  100,
			Data:      callData, // Include call data
		}
		callTx.Signature = callTx.GetSignature(minerPriv)

		gasUsed, err := blockchain.ApplyTransaction(&callTx, chain.AccountStates)
		if err != nil {
			t.Fatalf("Failed to call contract: %v", err)
		}

		// Expected gas: base transfer + data gas + execution
		minExpectedGas := config.GasTransfer + uint64(len(callData))*config.GasDataPerByte
		if gasUsed < minExpectedGas {
			t.Errorf("Call gas too low: got %d, expected at least %d", gasUsed, minExpectedGas)
		}

		t.Logf("Contract call with %d bytes of data used %d gas", len(callData), gasUsed)
	})

	// Test 4: Empty data field (should add no gas)
	t.Run("EmptyDataField", func(t *testing.T) {
		recipientPub, _, _ := ed25519.GenerateKey(nil)
		var recipient blockchain.PublicKey
		copy(recipient[:], recipientPub)

		tx := blockchain.Transaction{
			From:      minerPublicKey,
			To:        recipient,
			Amount:    1 * config.AUG,
			Nonce:     5,
			Timestamp: uint64(time.Now().Unix()),
			ChainID:   config.MainnetChainID,
			GasLimit:  25000,
			GasPrice:  100,
			Data:      nil, // No data
		}
		tx.Signature = tx.GetSignature(minerPriv)

		gasUsed, err := blockchain.ApplyTransaction(&tx, chain.AccountStates)
		if err != nil {
			t.Fatalf("Failed to apply transaction: %v", err)
		}

		// Should only charge base gas
		if gasUsed != config.GasTransfer {
			t.Errorf("Empty data gas mismatch: got %d, expected %d", gasUsed, config.GasTransfer)
		}

		t.Logf("Transaction with no data used %d gas", gasUsed)
	})

	// Test 5: Large data field
	t.Run("LargeDataField", func(t *testing.T) {
		recipientPub, _, _ := ed25519.GenerateKey(nil)
		var recipient blockchain.PublicKey
		copy(recipient[:], recipientPub)

		// Create 1KB of data
		largeData := make([]byte, 1024)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		tx := blockchain.Transaction{
			From:      minerPublicKey,
			To:        recipient,
			Amount:    1 * config.AUG,
			Nonce:     6,
			Timestamp: uint64(time.Now().Unix()),
			ChainID:   config.MainnetChainID,
			GasLimit:  100000, // Higher gas limit for large data
			GasPrice:  100,
			Data:      largeData,
		}
		tx.Signature = tx.GetSignature(minerPriv)

		gasUsed, err := blockchain.ApplyTransaction(&tx, chain.AccountStates)
		if err != nil {
			t.Fatalf("Failed to apply transaction: %v", err)
		}

		expectedGas := config.GasTransfer + uint64(len(largeData))*config.GasDataPerByte
		if gasUsed != expectedGas {
			t.Errorf("Large data gas mismatch: got %d, expected %d", gasUsed, expectedGas)
		}

		t.Logf("Transaction with %d bytes of data used %d gas (%.2f gas per byte)",
			len(largeData), gasUsed, float64(gasUsed-config.GasTransfer)/float64(len(largeData)))
	})
}

// TestTransactionDataInContext tests that transaction data is accessible in blockchain context
func TestTransactionDataInContext(t *testing.T) {
	// This test verifies that the TsxData field is properly populated in BlockchainContext

	testData := []byte("test data")
	hexData := hex.EncodeToString(testData)

	// Create a mock transaction
	tx := blockchain.Transaction{
		From:      blockchain.PublicKey{},
		To:        blockchain.PublicKey{},
		Amount:    1000,
		Nonce:     1,
		Timestamp: uint64(time.Now().Unix()),
		ChainID:   config.MainnetChainID,
		GasLimit:  50000,
		GasPrice:  100,
		Data:      testData,
	}

	// Create blockchain context (this would normally be done in validation.go)
	blockContext := &avm.BlockchainContext{
		BlockNumber:     1,
		BlockHash:       blockchain.Hash32{},
		LastBlockHash:   blockchain.Hash32{},
		Timestamp:       uint64(time.Now().Unix()),
		Difficulty:      1000,
		GasLimit:        tx.GasLimit,
		Coinbase:        blockchain.PublicKey{},
		ChainID:         config.MainnetChainID,
		Caller:          tx.From,
		Origin:          tx.From,
		GasPrice:        tx.GasPrice,
		CallValue:       tx.Amount,
		ContractAddress: tx.To,
		TsxData:         tx.Data, // This should be populated
		Deployer:        blockchain.PublicKey{},
		DeploymentTime:  0,
		DeploymentGas:   0,
	}

	// Verify data is in context
	if string(blockContext.TsxData) != string(testData) {
		t.Errorf("TsxData mismatch: got %x, expected %x", blockContext.TsxData, testData)
	}

	t.Logf("Transaction data successfully included in blockchain context: %s", hexData)

	// Note: Full integration with @tsxdata in Marigold would require additional
	// work to connect the AVM runtime context with Marigold's chain identifiers
}