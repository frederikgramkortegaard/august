package blockchain

import (
	"crypto/ed25519"
	"fmt"
)

func validateBlockHeaderIsGenesis(header *BlockHeader) bool {
	// Ensure the first block is genesis
	genesisHash := HashBlockHeader(&GenesisBlock.Header)
	firstBlockHash := HashBlockHeader(header)

	// Direct comparison for [32]byte arrays
	if genesisHash != firstBlockHash {
		fmt.Println("Block is not genesis")
		return false
	}
	return true
}

// validateBlockStructure validates block structure without state (PoW, hashes, timestamps)
func validateBlockStructure(block *Block, prevBlock *Block) bool {
	// 1. Previous Hash Linking
	prevHash := HashBlockHeader(&prevBlock.Header)
	if block.Header.PreviousHash != prevHash {
		fmt.Println("Failed Previous Hash Linking")
		return false
	}

	// 2. Proof Of Work
	hash := HashBlockHeader(&block.Header)
	if !BlockHashMeetsDifficulty(hash) {
		fmt.Println("Block does not meet difficulty", Difficulty)
		fmt.Printf("Hash: %x\n", hash[:])
		return false
	}

	// 3. Merkle Root
	merkle := MerkleTransactions(block.Transactions)
	if merkle != block.Header.MerkleRoot {
		fmt.Println("Merkle root is not correct")
		return false
	}

	// 4. Timestamp Sanity
	if block.Header.Timestamp <= prevBlock.Header.Timestamp {
		fmt.Println("Timestamp is not in order")
		return false
	}

	fmt.Println("Block structure validation passed")
	return true
}

// validateTransactionSignature validates just the cryptographic signature
func validateTransactionSignature(tsx *Transaction) bool {
	signingData := GetSigningBytesFromTransaction(tsx)
	publicKey := tsx.From[:]
	signature := tsx.Signature[:]
	return ed25519.Verify(publicKey, signingData, signature)
}

// validateAndApplyTransaction validates a single transaction against current state and applies it
func validateAndApplyTransaction(tsx *Transaction, accountStates map[PublicKey]*AccountState) bool {
	// Coinbase transactions - just apply
	if tsx.From == (PublicKey{}) {
		fmt.Println("Processing coinbase transaction")
		
		// Credit the recipient
		if toState, ok := accountStates[tsx.To]; ok {
			toState.Balance += tsx.Amount
		} else {
			accountStates[tsx.To] = &AccountState{
				Balance: tsx.Amount,
				Address: tsx.To,
				Nonce:   0,
			}
			fmt.Printf("Created new account via coinbase: %x\n", tsx.To[:])
		}
		return true
	}

	// Regular transactions - validate first, then apply
	
	// 1. Signature validation
	if !validateTransactionSignature(tsx) {
		fmt.Println("Invalid transaction signature")
		return false
	}

	// 2. Check sender account exists and has sufficient balance
	fromState, exists := accountStates[tsx.From]
	if !exists {
		fmt.Printf("Sender account does not exist: %x\n", tsx.From[:])
		return false
	}

	if fromState.Balance < tsx.Amount {
		fmt.Printf("Insufficient balance: has %d, needs %d\n", fromState.Balance, tsx.Amount)
		return false
	}

	// 3. Nonce validation (prevent double-spend)
	if tsx.Nonce != (fromState.Nonce + 1) {
		fmt.Printf("Invalid nonce: expected %d, got %d\n", fromState.Nonce+1, tsx.Nonce)
		return false
	}

	// 4. All validation passed - apply the transaction
	
	// Deduct from sender
	fromState.Balance -= tsx.Amount
	fromState.Nonce += 1

	// Credit recipient
	if toState, ok := accountStates[tsx.To]; ok {
		toState.Balance += tsx.Amount
	} else {
		accountStates[tsx.To] = &AccountState{
			Balance: tsx.Amount,
			Address: tsx.To,
			Nonce:   0,
		}
		fmt.Printf("Created new account: %x\n", tsx.To[:])
	}

	return true
}

// validateAndApplyBlock validates block structure, then validates and applies each transaction
func validateAndApplyBlock(block *Block, prevBlock *Block, accountStates map[PublicKey]*AccountState) bool {
	// First validate block structure (PoW, hashes, etc.)
	if !validateBlockStructure(block, prevBlock) {
		return false
	}

	// Then validate and apply each transaction incrementally
	for i, tsx := range block.Transactions {
		if !validateAndApplyTransaction(&tsx, accountStates) {
			fmt.Printf("Transaction %d failed validation\n", i)
			return false
		}
	}

	fmt.Println("Block validation and application completed successfully")
	return true
}

// ValidateAndBuildChain validates an entire chain and builds the account states
func ValidateAndBuildChain(blocks []*Block) *Chain {
	if len(blocks) == 0 {
		fmt.Println("Chain has no blocks")
		return nil
	}

	// Create chain with empty account states
	chain := &Chain{
		Blocks:        blocks,
		AccountStates: make(map[PublicKey]*AccountState),
	}

	// Genesis block validation
	if !validateBlockHeaderIsGenesis(&blocks[0].Header) {
		fmt.Println("Genesis block validation failed")
		return nil
	}

	// Process genesis block transactions (usually just coinbase)
	for _, tsx := range blocks[0].Transactions {
		if !validateAndApplyTransaction(&tsx, chain.AccountStates) {
			fmt.Println("Genesis block transaction failed")
			return nil
		}
	}

	// Process remaining blocks
	for i := 1; i < len(blocks); i++ {
		fmt.Printf("Validating block %d\n", i)
		
		if !validateAndApplyBlock(blocks[i], blocks[i-1], chain.AccountStates) {
			fmt.Printf("Block %d validation failed\n", i)
			return nil
		}
	}

	fmt.Printf("Chain validation successful! Final state has %d accounts\n", len(chain.AccountStates))
	return chain
}
