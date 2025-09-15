package blockchain

import (
	"crypto/ed25519"
	"fmt"
	"log"
)

// DebugLogging controls whether to output verbose validation logs
var DebugLogging = false

// debugLog logs a message only if debug logging is enabled
func debugLog(format string, args ...interface{}) {
	if DebugLogging {
		log.Printf(format, args...)
	}
}

// ErrMissingParent is returned when a block's parent is not found in the chain
type ErrMissingParent struct {
	Hash Hash32
}

func (e ErrMissingParent) Error() string { return "missing parent" }

// ErrSwitchChain is returned when a block triggers a chain reorganization
type ErrSwitchChain struct {
	Block          *Block
	CommonAncestor *Block
}

func (e ErrSwitchChain) Error() string { return "chain switch" }

// ValidateHeaderStructure validates a single block header without chain context
// This is used by networking layer to validate headers from peers
func ValidateHeaderStructure(header *BlockHeader) error {
	// 1. Proof of Work - check if hash meets the target specified in header
	hash := HashBlockHeader(header)
	if !BlockHashMeetsDifficulty(hash, header.Bits) {
		difficulty := CalculateDifficulty(header.Bits)
		diffFloat, _ := difficulty.Float64()
		return fmt.Errorf("header does not meet target difficulty %.2f, hash: %x", diffFloat, hash[:8])
	}

	// 2. Basic field validation
	if header.Height == 0 {
		// Genesis block validation
		genesisHash := HashBlockHeader(&GenesisBlock.Header)
		headerHash := HashBlockHeader(header)
		if genesisHash != headerHash {
			return fmt.Errorf("invalid genesis header")
		}
	}

	// 3. Timestamp sanity (not too far in future)
	// TODO: Add timestamp validation if needed

	return nil
}

// ValidateBlockStructure validates a single block without chain context
// This is used by networking layer to validate blocks from peers
func ValidateBlockStructure(block *Block) error {
	// 1. Validate the header first
	if err := ValidateHeaderStructure(&block.Header); err != nil {
		return fmt.Errorf("invalid block header: %w", err)
	}

	// 2. Merkle root validation
	merkle := MerkleTransactions(block.Transactions)
	if merkle != block.Header.MerkleRoot {
		return fmt.Errorf("merkle root mismatch")
	}

	// 3. Basic transaction structure validation
	for i, tx := range block.Transactions {
		if i == 0 {
			// Coinbase transaction - should have zero hash as From
			if tx.From != (PublicKey{}) {
				// Actually, only check this for non-coinbase
			}
		} else {
			// Regular transaction - validate signature structure
			if !validateTransactionSignature(&tx) {
				return fmt.Errorf("invalid transaction signature in transaction %d", i)
			}
		}
	}

	return nil
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
		// Check if we have the parent block using O(1) lookup
		var parentExists bool
		prevBlock, parentExists = chain.GetBlockByHash(block.Header.PreviousHash)

		// Parent block not found - this is an orphan
		if !parentExists {
			return ErrMissingParent{Hash: block.Header.PreviousHash}
		}

		// Check if this is a fork (parent exists but is not the tip)
		if chain.Tip != nil && HashBlockHeader(&chain.Tip.Header) != block.Header.PreviousHash {
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
	debugLog("VALIDATION\tValidating transaction signature from %x", tsx.From[:8])
	signingData := GetSigningBytesFromTransaction(tsx)
	publicKey := tsx.From[:]
	signature := tsx.Signature[:]
	valid := ed25519.Verify(publicKey, signingData, signature)
	if !valid {
		debugLog("VALIDATION\tInvalid signature for transaction from %x", tsx.From[:8])
	} else {
		debugLog("VALIDATION\tValid signature for transaction from %x", tsx.From[:8])
	}
	return valid
}

// ValidateAndApplyTransaction validates and applies a single transaction to account states
// This is a convenience function that combines separate validation and application
func ValidateAndApplyTransaction(tsx *Transaction, accountStates map[PublicKey]*AccountState) bool {
	// First validate
	if err := ValidateTransaction(tsx, accountStates); err != nil {
		debugLog("VALIDATION\tTRANSACTION REJECTED: %v", err)
		return false
	}

	// Then apply
	debugLog("VALIDATION\tTRANSACTION ACCEPTED: Applying transaction")
	ApplyTransaction(tsx, accountStates)
	return true
}

// ValidateTransaction validates a transaction against current state WITHOUT applying changes
func ValidateTransaction(tsx *Transaction, accountStates map[PublicKey]*AccountState) error {

	// Validate amounts are within range
	if err := ValidateAmount(tsx.Amount); err != nil {
		return fmt.Errorf("invalid transaction amount: %w", err)
	}
	if err := ValidateAmount(tsx.Fee); err != nil {
		return fmt.Errorf("invalid transaction fee: %w", err)
	}

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

	// Check for overflow in amount + fee using safe arithmetic
	total, err := SafeAdd(tsx.Amount, tsx.Fee)
	if err != nil {
		return fmt.Errorf("transaction amount + fee overflow: %w", err)
	}

	if fromState.Balance < total {
		return fmt.Errorf("insufficient balance: has %d, needs %d", fromState.Balance, total)
	}

	// 3. Nonce validation (prevent double-spend)
	if tsx.Nonce != (fromState.Nonce + 1) {
		return fmt.Errorf("invalid nonce: expected %d, got %d", fromState.Nonce+1, tsx.Nonce)
	}

	// All validation passed - transaction is valid
	return nil
}

// ValidateStandaloneTransaction validates a transaction against current chain state for mempool
// This is used by the networking layer when receiving broadcast transactions before they're in blocks
func ValidateStandaloneTransaction(tsx *Transaction, chain *Chain) error {
	// Use the current chain's account states for validation
	return ValidateTransaction(tsx, chain.AccountStates)
}

// ApplyTransaction applies a valid transaction to account states (assumes already validated)
func ApplyTransaction(tsx *Transaction, accountStates map[PublicKey]*AccountState) {
	debugLog("APPLY\tApplying transaction: %x -> %x, amount=%d, nonce=%d",
		tsx.From[:4], tsx.To[:4], tsx.Amount, tsx.Nonce)

	// Coinbase transactions
	if tsx.From == (PublicKey{}) {
		debugLog("APPLY\tProcessing coinbase transaction")

		// Credit the recipient
		if toState, ok := accountStates[tsx.To]; ok {
			newBalance, err := SafeAdd(toState.Balance, tsx.Amount)
			if err != nil {
				// This should never happen in practice due to validation, but safety check
				panic(fmt.Sprintf("coinbase balance overflow: %v", err))
			}
			toState.Balance = newBalance
		} else {
			accountStates[tsx.To] = &AccountState{
				Balance: tsx.Amount,
				Address: tsx.To,
				Nonce:   0,
			}
			fmt.Printf("Created new account via coinbase: %x\n", tsx.To[:])
		}
		return
	}

	// Regular transactions
	fromState := accountStates[tsx.From] // Should exist (validated earlier)

	// Deduct from sender (amount + fee) - overflow already checked in validation
	total, err := SafeAdd(tsx.Amount, tsx.Fee)
	if err != nil {
		panic(fmt.Sprintf("transaction total overflow during apply: %v", err))
	}
	newFromBalance, err := SafeSubtract(fromState.Balance, total)
	if err != nil {
		panic(fmt.Sprintf("sender balance underflow during apply: %v", err))
	}
	fromState.Balance = newFromBalance
	fromState.Nonce += 1
	debugLog("APPLY\tSender %x new balance=%d, nonce=%d", tsx.From[:4], fromState.Balance, fromState.Nonce)

	// Credit recipient
	if toState, ok := accountStates[tsx.To]; ok {
		newToBalance, err := SafeAdd(toState.Balance, tsx.Amount)
		if err != nil {
			panic(fmt.Sprintf("recipient balance overflow during apply: %v", err))
		}
		toState.Balance = newToBalance
	} else {
		accountStates[tsx.To] = &AccountState{
			Balance: tsx.Amount,
			Address: tsx.To,
			Nonce:   0,
		}
		fmt.Printf("Created new account: %x\n", tsx.To[:])
	}
}

// ValidateBlock validates a block and its transactions WITHOUT applying changes
func ValidateBlock(block *Block, chain *Chain) error {
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

	// Create a copy of account states for validation without modifying original
	accountStatesCopy := make(map[PublicKey]*AccountState)
	for k, v := range chain.AccountStates {
		accountStatesCopy[k] = &AccountState{
			Address: v.Address,
			Balance: v.Balance,
			Nonce:   v.Nonce,
		}
	}

	var tsxfeesum uint64 = 0
	// Validate each transaction without applying changes
	for i, tsx := range block.Transactions {
		// Check for fee sum overflow
		newFeeSum, err := SafeAdd(tsxfeesum, tsx.Fee)
		if err != nil {
			return fmt.Errorf("transaction fees sum overflow: %w", err)
		}
		tsxfeesum = newFeeSum

		if err := ValidateTransaction(&tsx, accountStatesCopy); err != nil {
			return fmt.Errorf("transaction %d failed validation: %w", i, err)
		}
		// Apply to the copy for subsequent transaction validation
		ApplyTransaction(&tsx, accountStatesCopy)
	}

	// Validate Coinbase tsx amount is Fee's + Block Reward
	expectedCoinbaseAmount, err := SafeAdd(tsxfeesum, BlockReward)
	if err != nil {
		return fmt.Errorf("coinbase amount calculation overflow: %w", err)
	}
	if block.Transactions[0].Amount != expectedCoinbaseAmount {
		return fmt.Errorf("Coinbase transaction is not Transaction Fee's + BlockReward")
	}

	return nil
}

// ApplyBlock applies a validated block to the chain (assumes already validated)
func ApplyBlock(block *Block, chain *Chain) {
	// Apply each transaction to the chain state
	for _, tsx := range block.Transactions {
		ApplyTransaction(&tsx, chain.AccountStates)
	}

	// Add the block to the chain and update tip/index
	chain.AddBlock(block)
}

// ValidateAndApplyBlock validates block structure, applies transactions, and adds block to chain
func ValidateAndApplyBlock(block *Block, chain *Chain) error {
	// First validate
	if err := ValidateBlock(block, chain); err != nil {
		return err
	}

	// Then apply
	ApplyBlock(block, chain)
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

		// PoW validation - check if hash meets the target specified in header
		if err := ValidateHeaderStructure(&headers[i]); err != nil {
			debugLog("VALIDATION\tHeader %d failed structure validation: %v", i, err)
			return false
		}

		// Timestamp validation - must be greater than previous block
		if headers[i].Timestamp <= headers[i-1].Timestamp {
			debugLog("VALIDATION\tHeader %d timestamp not greater than previous", i)
			return false
		}
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

	// Initialize with genesis account (FirstUser gets 10 AUG)
	validationChain.AccountStates[FirstUser] = &AccountState{
		Address: FirstUser,
		Balance: 10 * AUG,
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
