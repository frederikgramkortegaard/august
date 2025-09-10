package tests

import (
	"august/blockchain"
	"crypto/ed25519"
	"testing"
)

func TestTransactionFeesValidation(t *testing.T) {
	// Create a test chain starting from genesis
	chain := &blockchain.Chain{
		Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}

	// Process genesis block to setup initial account states
	for _, tsx := range blockchain.GenesisBlock.Transactions {
		if !blockchain.ValidateAndApplyTransaction(&tsx, chain.AccountStates) {
			t.Fatal("Failed to process genesis transaction")
		}
	}

	// Generate keys for miner, sender and recipient
	minerPubKey, _, _ := ed25519.GenerateKey(nil)
	senderPubKey, senderPrivKey, _ := ed25519.GenerateKey(nil)
	recipientPubKey, _, _ := ed25519.GenerateKey(nil)

	// Give sender some initial balance via a block
	setupCoinbase := blockchain.Transaction{
		From:   blockchain.PublicKey{},
		To:     blockchain.PublicKey(senderPubKey),
		Amount: blockchain.BlockReward, // Miner reward goes to sender for test setup
		Nonce:  0,
	}

	setupBlock, err := blockchain.NewBlock(blockchain.BlockCreationParams{
		Version:      1,
		PreviousHash: blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header),
		Height:       1,
		PreviousWork: blockchain.GenesisBlock.Header.TotalWork,
		Coinbase:     setupCoinbase,
		Transactions: []blockchain.Transaction{},
		Timestamp:    blockchain.GenesisBlock.Header.Timestamp + 1,
		TargetBits:   blockchain.TestTargetCompact,
	})
	if err != nil {
		t.Fatalf("Failed to create setup block: %v", err)
	}

	if err := blockchain.ValidateAndApplyBlock(&setupBlock, chain); err != nil {
		t.Fatalf("Failed to apply setup block: %v", err)
	}

	// Test Case 1: Correct fee calculation
	t.Run("CorrectFeeCalculation", func(t *testing.T) {
		// Create transactions with fees
		tx1 := createSignedTransaction(senderPubKey, senderPrivKey, recipientPubKey, 10, 2, 1) // 2 fee
		tx2 := createSignedTransaction(senderPubKey, senderPrivKey, recipientPubKey, 10, 3, 2) // 3 fee
		
		totalFees := uint64(5) // 2 + 3
		expectedCoinbaseAmount := blockchain.BlockReward + totalFees

		// Create coinbase with correct amount
		coinbase := blockchain.Transaction{
			From:   blockchain.PublicKey{},
			To:     blockchain.PublicKey(minerPubKey),
			Amount: expectedCoinbaseAmount,
			Nonce:  0,
		}

		// Create block with transactions
		block, err := blockchain.NewBlock(blockchain.BlockCreationParams{
			Version:      1,
			PreviousHash: blockchain.HashBlockHeader(&chain.Blocks[len(chain.Blocks)-1].Header),
			Height:       uint64(len(chain.Blocks)),
			PreviousWork: chain.Blocks[len(chain.Blocks)-1].Header.TotalWork,
			Coinbase:     coinbase,
			Transactions: []blockchain.Transaction{tx1, tx2},
			Timestamp:    chain.Blocks[len(chain.Blocks)-1].Header.Timestamp + 1,
			TargetBits:   blockchain.TestTargetCompact,
		})

		if err != nil {
			t.Fatalf("Should create block with correct fees: %v", err)
		}

		// Validate and apply block
		testChain := cloneChain(chain)
		if err := blockchain.ValidateAndApplyBlock(&block, testChain); err != nil {
			t.Fatalf("Should validate block with correct fees: %v", err)
		}

		// Verify miner received correct reward
		minerState := testChain.AccountStates[blockchain.PublicKey(minerPubKey)]
		if minerState.Balance != expectedCoinbaseAmount {
			t.Errorf("Miner balance incorrect: got %d, want %d", minerState.Balance, expectedCoinbaseAmount)
		}

		// Verify sender paid fees (fees are deducted along with amount in validation.go line 166)
		senderState := testChain.AccountStates[blockchain.PublicKey(senderPubKey)]
		expectedSenderBalance := uint64(blockchain.BlockReward - (10 + 2) - (10 + 3)) // initial - (amt1+fee1) - (amt2+fee2)
		if senderState.Balance != expectedSenderBalance {
			t.Errorf("Sender balance incorrect: got %d, want %d", senderState.Balance, expectedSenderBalance)
		}
	})

	// Test Case 2: Incorrect coinbase amount (too high)
	t.Run("CoinbaseTooHigh", func(t *testing.T) {
		tx := createSignedTransaction(senderPubKey, senderPrivKey, recipientPubKey, 5, 2, 3)
		
		// Create coinbase with too much reward
		coinbase := blockchain.Transaction{
			From:   blockchain.PublicKey{},
			To:     blockchain.PublicKey(minerPubKey),
			Amount: blockchain.BlockReward + 10, // Should be +2, not +10
			Nonce:  0,
		}

		// Try to create block - should fail
		_, err := blockchain.NewBlock(blockchain.BlockCreationParams{
			Version:      1,
			PreviousHash: blockchain.HashBlockHeader(&chain.Blocks[len(chain.Blocks)-1].Header),
			Height:       uint64(len(chain.Blocks)),
			PreviousWork: chain.Blocks[len(chain.Blocks)-1].Header.TotalWork,
			Coinbase:     coinbase,
			Transactions: []blockchain.Transaction{tx},
			Timestamp:    chain.Blocks[len(chain.Blocks)-1].Header.Timestamp + 1,
			TargetBits:   blockchain.TestTargetCompact,
		})

		if err == nil {
			t.Fatal("Should reject block with excessive coinbase amount")
		}
	})

	// Test Case 3: Incorrect coinbase amount (too low)
	t.Run("CoinbaseTooLow", func(t *testing.T) {
		tx := createSignedTransaction(senderPubKey, senderPrivKey, recipientPubKey, 5, 2, 4)
		
		// Create coinbase with too little reward
		coinbase := blockchain.Transaction{
			From:   blockchain.PublicKey{},
			To:     blockchain.PublicKey(minerPubKey),
			Amount: blockchain.BlockReward + 1, // Should be +2, not +1
			Nonce:  0,
		}

		// Try to create block - should fail
		_, err := blockchain.NewBlock(blockchain.BlockCreationParams{
			Version:      1,
			PreviousHash: blockchain.HashBlockHeader(&chain.Blocks[len(chain.Blocks)-1].Header),
			Height:       uint64(len(chain.Blocks)),
			PreviousWork: chain.Blocks[len(chain.Blocks)-1].Header.TotalWork,
			Coinbase:     coinbase,
			Transactions: []blockchain.Transaction{tx},
			Timestamp:    chain.Blocks[len(chain.Blocks)-1].Header.Timestamp + 1,
			TargetBits:   blockchain.TestTargetCompact,
		})

		if err == nil {
			t.Fatal("Should reject block with insufficient coinbase amount")
		}
	})

	// Test Case 4: Zero fees
	t.Run("ZeroFees", func(t *testing.T) {
		// Transaction with zero fee
		tx := createSignedTransaction(senderPubKey, senderPrivKey, recipientPubKey, 5, 0, 1)
		
		// Coinbase should be just block reward
		coinbase := blockchain.Transaction{
			From:   blockchain.PublicKey{},
			To:     blockchain.PublicKey(minerPubKey),
			Amount: blockchain.BlockReward,
			Nonce:  0,
		}

		block, err := blockchain.NewBlock(blockchain.BlockCreationParams{
			Version:      1,
			PreviousHash: blockchain.HashBlockHeader(&chain.Blocks[len(chain.Blocks)-1].Header),
			Height:       uint64(len(chain.Blocks)),
			PreviousWork: chain.Blocks[len(chain.Blocks)-1].Header.TotalWork,
			Coinbase:     coinbase,
			Transactions: []blockchain.Transaction{tx},
			Timestamp:    chain.Blocks[len(chain.Blocks)-1].Header.Timestamp + 1,
			TargetBits:   blockchain.TestTargetCompact,
		})

		if err != nil {
			t.Fatalf("Should create block with zero fees: %v", err)
		}

		// Validate block
		testChain := cloneChain(chain)
		if err := blockchain.ValidateAndApplyBlock(&block, testChain); err != nil {
			t.Fatalf("Should validate block with zero fees: %v", err)
		}
	})

	// Test Case 5: No transactions (only coinbase)
	t.Run("OnlyCoinbase", func(t *testing.T) {
		// Coinbase with just block reward, no transactions
		coinbase := blockchain.Transaction{
			From:   blockchain.PublicKey{},
			To:     blockchain.PublicKey(minerPubKey),
			Amount: blockchain.BlockReward,
			Nonce:  0,
		}

		block, err := blockchain.NewBlock(blockchain.BlockCreationParams{
			Version:      1,
			PreviousHash: blockchain.HashBlockHeader(&chain.Blocks[len(chain.Blocks)-1].Header),
			Height:       uint64(len(chain.Blocks)),
			PreviousWork: chain.Blocks[len(chain.Blocks)-1].Header.TotalWork,
			Coinbase:     coinbase,
			Transactions: []blockchain.Transaction{}, // No other transactions
			Timestamp:    chain.Blocks[len(chain.Blocks)-1].Header.Timestamp + 1,
			TargetBits:   blockchain.TestTargetCompact,
		})

		if err != nil {
			t.Fatalf("Should create block with only coinbase: %v", err)
		}

		// Validate block
		testChain := cloneChain(chain)
		if err := blockchain.ValidateAndApplyBlock(&block, testChain); err != nil {
			t.Fatalf("Should validate block with only coinbase: %v", err)
		}
	})
}

// Helper function to create a signed transaction
func createSignedTransaction(fromPub ed25519.PublicKey, fromPriv ed25519.PrivateKey, 
	toPub ed25519.PublicKey, amount uint64, fee uint64, nonce uint64) blockchain.Transaction {
	
	tx := blockchain.Transaction{
		From:   blockchain.PublicKey(fromPub),
		To:     blockchain.PublicKey(toPub),
		Amount: amount,
		Fee:    fee,
		Nonce:  nonce,
	}
	
	// Sign the transaction
	signingBytes := blockchain.GetSigningBytesFromTransaction(&tx)
	signature := ed25519.Sign(fromPriv, signingBytes)
	copy(tx.Signature[:], signature)
	
	return tx
}

// Helper function to clone a chain for testing
func cloneChain(original *blockchain.Chain) *blockchain.Chain {
	newChain := &blockchain.Chain{
		Blocks:        make([]*blockchain.Block, len(original.Blocks)),
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	
	// Copy blocks
	copy(newChain.Blocks, original.Blocks)
	
	// Deep copy account states
	for key, state := range original.AccountStates {
		newChain.AccountStates[key] = &blockchain.AccountState{
			Balance: state.Balance,
			Address: state.Address,
			Nonce:   state.Nonce,
		}
	}
	
	return newChain
}