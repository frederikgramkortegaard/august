package blockchain

import (
	"crypto/ed25519"
	"fmt"
	"log"
)

// ErrMissingParent is returned when a block's parent is not found in the chain
type ErrMissingParent struct {
	Hash Hash32
}

func (e ErrMissingParent) Error() string {
	return fmt.Sprintf("missing parent block: %x", e.Hash[:8])
}

// ErrSwitchChain is returned when a block triggers a chain reorganization
type ErrSwitchChain struct {
	Block          *Block
	CommonAncestor *Block
}

func (e ErrSwitchChain) Error() string {
	return "chain reorganization required"
}

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

// validateBlockStructure validates block structure using chain context for difficulty
func validateBlockStructure(block *Block, chain *Chain) error {

	var prevBlock *Block
	currentHeight := len(chain.Blocks)

	// Genesis block validation
	if currentHeight == 0 {
		if !validateBlockHeaderIsGenesis(&block.Header) {
			return fmt.Errorf("block is not genesis")
		}
	} else {
		// Check if we have the parent block
		parentExists := false
		for _, chainBlock := range chain.Blocks {
			if HashBlockHeader(&chainBlock.Header) == block.Header.PreviousHash {
				prevBlock = chainBlock
				parentExists = true
				break
			}
		}

		// Parent block not found - this is an orphan
		if !parentExists {
			return ErrMissingParent{Hash: block.Header.PreviousHash}
		}

		// Check if this is a fork (parent exists but is not the tip)
		if len(chain.Blocks) > 0 && HashBlockHeader(&chain.Blocks[len(chain.Blocks)-1].Header) != block.Header.PreviousHash {
			// This is a fork - let ValidateAndApplyBlock handle chain switching
			return ErrSwitchChain{Block: block, CommonAncestor: prevBlock}
		}
	}

	// 2. Proof Of Work - check if hash meets the target specified in block header
	hash := HashBlockHeader(&block.Header)
	if !BlockHashMeetsDifficulty(hash, block.Header.Bits) {
		difficulty := CalculateDifficulty(block.Header.Bits)
		diffFloat, _ := difficulty.Float64()
		return fmt.Errorf("block does not meet target difficulty %.2f, hash: %x", diffFloat, hash[:8])
	}
	
	// 2b. Verify the target bits are correct for this height
	expectedBits := GetTargetBits(currentHeight, chain.Blocks)
	if block.Header.Bits != expectedBits {
		return fmt.Errorf("incorrect target bits: got %x, expected %x", block.Header.Bits, expectedBits)
	}

	// 3. Merkle Root
	merkle := MerkleTransactions(block.Transactions)
	if merkle != block.Header.MerkleRoot {
		return fmt.Errorf("merkle root is not correct")
	}

	// 4. Timestamp Sanity
	if prevBlock != nil && (block.Header.Timestamp <= prevBlock.Header.Timestamp) {
		return fmt.Errorf("timestamp is not in order")
	}

	return nil
}

// validateTransactionSignature validates just the cryptographic signature
func validateTransactionSignature(tsx *Transaction) bool {
	log.Printf("VALIDATION\tValidating transaction signature from %x", tsx.From[:8])
	signingData := GetSigningBytesFromTransaction(tsx)
	publicKey := tsx.From[:]
	signature := tsx.Signature[:]
	valid := ed25519.Verify(publicKey, signingData, signature)
	if !valid {
		log.Printf("VALIDATION\tInvalid signature for transaction from %x", tsx.From[:8])
	} else {
		log.Printf("VALIDATION\tValid signature for transaction from %x", tsx.From[:8])
	}
	return valid
}

// ValidateAndApplyTransaction validates and applies a single transaction to account states
func ValidateAndApplyTransaction(tsx *Transaction, accountStates map[PublicKey]*AccountState) bool {
	log.Printf("VALIDATION\tValidating transaction: %x -> %x, amount=%d, nonce=%d",
		tsx.From[:4], tsx.To[:4], tsx.Amount, tsx.Nonce)

	// Coinbase transactions - just apply
	if tsx.From == (PublicKey{}) {
		log.Printf("VALIDATION\tProcessing coinbase transaction")

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
		log.Printf("VALIDATION\tTRANSACTION REJECTED: Invalid signature")
		return false
	}

	// 2. Check sender account exists and has sufficient balance
	fromState, exists := accountStates[tsx.From]
	if !exists {
		log.Printf("VALIDATION\tTRANSACTION REJECTED: Sender account does not exist: %x", tsx.From[:8])
		return false
	}

	log.Printf("VALIDATION\tSender %x has balance=%d, nonce=%d", tsx.From[:4], fromState.Balance, fromState.Nonce)

	if fromState.Balance < tsx.Amount+tsx.Fee {
		log.Printf("VALIDATION\tTRANSACTION REJECTED: Insufficient balance: has %d, needs %d", fromState.Balance, tsx.Amount+tsx.Fee)
		return false
	}

	// 3. Nonce validation (prevent double-spend)
	if tsx.Nonce != (fromState.Nonce + 1) {
		log.Printf("VALIDATION\tTRANSACTION REJECTED: Invalid nonce: expected %d, got %d", fromState.Nonce+1, tsx.Nonce)
		return false
	}

	// 4. All validation passed - apply the transaction
	log.Printf("VALIDATION\tTRANSACTION ACCEPTED: Applying transaction")

	// Deduct from sender (amount + fee)
	fromState.Balance -= (tsx.Amount + tsx.Fee)
	fromState.Nonce += 1
	log.Printf("VALIDATION\tSender %x new balance=%d, nonce=%d", tsx.From[:4], fromState.Balance, fromState.Nonce)

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

// ValidateTransaction validates a transaction against current state WITHOUT applying changes
func ValidateTransaction(tsx *Transaction, accountStates map[PublicKey]*AccountState) error {

	// Coinbase transactions - always valid (no sender validation needed)
	if tsx.From == (PublicKey{}) {
		return nil
	}

	// Regular transactions - validate only

	// 1. Signature validation
	if !validateTransactionSignature(tsx) {
		return fmt.Errorf("invalid transaction signature")
	}

	// 2. Check sender account exists and has sufficient balance
	fromState, exists := accountStates[tsx.From]
	if !exists {
		return fmt.Errorf("sender account does not exist: %x", tsx.From[:8])
	}

	if fromState.Balance < tsx.Amount+tsx.Fee {
		return fmt.Errorf("insufficient balance: has %d, needs %d", fromState.Balance, tsx.Amount+tsx.Fee)
	}

	// 3. Nonce validation (prevent double-spend)
	if tsx.Nonce != (fromState.Nonce + 1) {
		return fmt.Errorf("invalid nonce: expected %d, got %d", fromState.Nonce+1, tsx.Nonce)
	}

	// All validation passed - transaction is valid
	return nil
}

// ValidateAndApplyBlock validates block structure, applies transactions, and adds block to chain
func ValidateAndApplyBlock(block *Block, chain *Chain) error {

	// First validate block structure (PoW, hashes, etc.)
	if err := validateBlockStructure(block, chain); err != nil {
		// Propagate ErrSwitchChain and ErrMissingParent directly, wrap other errors
		if _, ok := err.(ErrSwitchChain); ok {
			return err
		}
		if _, ok := err.(ErrMissingParent); ok {
			return err
		}
		return fmt.Errorf("block structure validation failed: %w", err)
	}

	var tsxfeesum uint64 = 0
	// Then validate and apply each transaction incrementally
	for i, tsx := range block.Transactions {
		tsxfeesum += tsx.Fee
		if !ValidateAndApplyTransaction(&tsx, chain.AccountStates) {
			return fmt.Errorf("transaction %d failed validation", i)
		}
	}

	// Validate Coinbase tsx amount is Fee's + Block Reward
	if block.Transactions[0].Amount != tsxfeesum+BlockReward {
		return fmt.Errorf("Coinbase transaction is not Transaction Fee's + BlockReward")
	}

	// Add the validated block to the chain
	chain.Blocks = append(chain.Blocks, block)

	return nil
}

// ValidateHeaderChain performs lightweight validation on a chain of headers
func ValidateHeaderChain(headers []BlockHeader) bool {
	if len(headers) == 0 {
		return false
	}

	// Basic validation - each header should link to the previous
	for i := 1; i < len(headers); i++ {
		prevHash := HashBlockHeader(&headers[i-1])
		if headers[i].PreviousHash != prevHash {
			return false
		}

		// Height should increment by 1
		if headers[i].Height != headers[i-1].Height+1 {
			return false
		}

		// TODO: Add PoW validation, difficulty checks, etc.
	}

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
		if !ValidateAndApplyTransaction(&tsx, chain.AccountStates) {
			fmt.Println("Genesis block transaction failed")
			return nil
		}
	}

	// Process remaining blocks using ValidateAndApplyBlock
	for i := 1; i < len(blocks); i++ {
		fmt.Printf("Validating block %d\n", i)

		if err := ValidateAndApplyBlock(blocks[i], chain); err != nil {
			fmt.Printf("Block %d validation failed: %v\n", i, err)
			return nil
		}
	}

	fmt.Printf("Chain validation successful! Final state has %d accounts\n", len(chain.AccountStates))
	return chain
}

// ValidateCompleteChain validates a complete chain by rebuilding state from genesis
// This is used for validating candidate chains before promotion
func ValidateCompleteChain(candidateChain *Chain) error {
	if len(candidateChain.Blocks) == 0 {
		return fmt.Errorf("empty chain")
	}

	// Create a fresh chain starting with genesis and rebuild state from scratch
	validationChain := &Chain{
		Blocks:        []*Block{candidateChain.Blocks[0]}, // Start with genesis
		AccountStates: make(map[PublicKey]*AccountState),
	}

	// Initialize with genesis account (FirstUser gets 10M coins)
	validationChain.AccountStates[FirstUser] = &AccountState{
		Address: FirstUser,
		Balance: 10_000_000,
		Nonce:   0,
	}

	// Validate each block sequentially
	for i := 1; i < len(candidateChain.Blocks); i++ {
		block := candidateChain.Blocks[i]
		if err := ValidateAndApplyBlock(block, validationChain); err != nil {
			return fmt.Errorf("block %d validation failed: %w", block.Header.Height, err)
		}
	}

	return nil
}

// BlockEvaluationResult represents the result of evaluating whether we want a block
type BlockEvaluationResult struct {
	ShouldRequest bool
	Reason        string
	Action        string // "sequence", "sync", "fork", "ignore"
}

// EvaluateBlockHeaderForSync determines if we should request a block based on its header
func EvaluateBlockHeaderForSync(header *BlockHeader, currentChain *Chain) BlockEvaluationResult {
	ourHeight := uint64(0)
	if len(currentChain.Blocks) > 0 {
		ourHeight = currentChain.Blocks[len(currentChain.Blocks)-1].Header.Height
	}

	if header.Height == ourHeight+1 {
		// This might be the next block in sequence
		if len(currentChain.Blocks) > 0 {
			ourTip := currentChain.Blocks[len(currentChain.Blocks)-1]
			if HashBlockHeader(&ourTip.Header) == header.PreviousHash {
				return BlockEvaluationResult{
					ShouldRequest: true,
					Reason:        "next block in sequence",
					Action:        "sequence",
				}
			}
		}
	} else if header.Height > ourHeight+1 {
		// This block is ahead of us - we might need to sync
		return BlockEvaluationResult{
			ShouldRequest: true,
			Reason:        "block ahead of us, might need sync",
			Action:        "sync",
		}
	} else if header.Height <= ourHeight {
		// Check if this could be a fork with more work
		if len(currentChain.Blocks) > 0 {
			ourTip := currentChain.Blocks[len(currentChain.Blocks)-1]
			if CompareWork(header.TotalWork, ourTip.Header.TotalWork) > 0 {
				return BlockEvaluationResult{
					ShouldRequest: true,
					Reason:        "potential fork with more work",
					Action:        "fork",
				}
			}
		}
	}

	return BlockEvaluationResult{
		ShouldRequest: false,
		Reason:        "not needed",
		Action:        "ignore",
	}
}
