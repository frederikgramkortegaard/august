package blockchain

import (
	"august/avm"
	"august/config"
	"august/types"
	"august/utils"
	"crypto/ed25519"
	"fmt"
	"log"
)

// ContractExecutor defines the interface for executing contracts
type ContractExecutor interface {
	ExecuteTransaction(tsx *Transaction, accountStates map[types.PublicKey]*AccountState, blockContext *avm.BlockchainContext) (uint64, error)
}

// Global executor instance - will be set by execution package
var contractExecutor ContractExecutor

// SetContractExecutor allows the execution package to register itself
func SetContractExecutor(executor ContractExecutor) {
	contractExecutor = executor
}

// ValidateHeaderStructure validates a single block header without chain context
// This is used by networking layer to validate headers from peers
func ValidateHeaderStructure(header *BlockHeader) error {
	// 1. Proof of Work - check if hash meets the target specified in header
	hash := header.GetHash()
	if !BlockHashMeetsDifficulty(hash, header.Bits) {
		difficulty := CalculateDifficulty(header.Bits)
		diffFloat, _ := difficulty.Float64()
		return fmt.Errorf("header does not meet target difficulty %.2f, hash: %x", diffFloat, hash[:8])
	}

	// 2. Basic field validation
	if header.Height == 0 {
		// Genesis block validation
		genesisHash := GenesisBlock.Header.GetHash()
		headerHash := header.GetHash()
		if genesisHash != headerHash {
			return fmt.Errorf("invalid genesis header")
		}
	}

	return nil
}

// ValidateHeaderTotalWork validates a header including TotalWork calculation
func ValidateHeaderTotalWork(header *BlockHeader, previousTotalWork string) error {
	// First do basic structure validation
	if err := ValidateHeaderStructure(header); err != nil {
		return err
	}

	// Validate TotalWork calculation
	if header.Height > 0 {
		blockWork := CalculateBlockWorkFromBits(header.Bits)
		expectedTotalWork := AddWork(previousTotalWork, blockWork)

		if header.TotalWork != expectedTotalWork {
			return fmt.Errorf("invalid total work: got %s, expected %s",
				header.TotalWork, expectedTotalWork)
		}
	}

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
			hash := tx.GetHash()
			if !ed25519.Verify(tx.From[:], hash[:], tx.Signature[:]) {
				return fmt.Errorf("invalid transaction signature in transaction %d", i)
			}
		}
	}

	return nil
}

// validateBlockStructure validates block structure using chain context for difficulty
func validateBlockStructure(block *Block, chain *Chain) error {

	var prevBlock *Block
	currentHeight := len(chain.Blocks)

	// Genesis block validation
	blockHash := block.Header.GetHash()
	genesisHash := GenesisBlock.Header.GetHash()
	isGenesisBlock := blockHash == genesisHash
	log.Printf("VALIDATION DEBUG: blockHash=%s, genesisHash=%s, isGenesis=%t", blockHash.String(), genesisHash.String(), isGenesisBlock)
	if isGenesisBlock {
		// Validate genesis-specific rules
		if block.Header.PreviousHash != (Hash32{}) {
			return fmt.Errorf("genesis block must have all-zeros parent hash")
		}
		if len(block.Transactions) != 1 {
			return fmt.Errorf("genesis block must have exactly one transaction")
		}
		coinbase := block.Transactions[0]
		if coinbase.From != (PublicKey{}) {
			return fmt.Errorf("genesis coinbase must have empty From address")
		}
		if coinbase.To != config.FirstUser {
			return fmt.Errorf("genesis coinbase must go to config.FirstUser")
		}
		if coinbase.Amount != 10*config.AUG {
			return fmt.Errorf("genesis coinbase amount must be exactly 10 AUG, got %d", coinbase.Amount)
		}
		return nil
	} else {
		// Check if we have the parent block using O(1) lookup
		var parentExists bool
		prevBlock, parentExists = chain.GetBlockByHash(block.Header.PreviousHash)

		// Parent block not found - this is an orphan
		if !parentExists {
			return ErrMissingParent{Hash: block.Header.PreviousHash}
		}

		// Check if this is a fork (parent exists but is not the tip)
		if chain.Tip != nil && chain.Tip.Header.GetHash() != block.Header.PreviousHash {
			// This is a fork - let ValidateAndApplyBlock handle chain switching
			tipHash := chain.Tip.Header.GetHash()
			log.Printf("FORK DETECTED: tip hash=%x, block.PreviousHash=%x, block height=%d, tip height=%d",
				tipHash[:8], block.Header.PreviousHash[:8], block.Header.Height, chain.Tip.Header.Height)
			return ErrSwitchChain{Block: block, CommonAncestor: prevBlock}
		}
	}

	// 2. Proof Of Work - check if hash meets the target specified in block header
	hash := block.Header.GetHash()
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

// ValidateAndApplyTransaction validates and applies a single transaction to account states
// This is a convenience function that combines separate validation and application
func ValidateAndApplyTransaction(tsx *Transaction, accountStates map[PublicKey]*AccountState) bool {
	// First validate
	if err := ValidateTransaction(tsx, accountStates); err != nil {
		log.Printf("VALIDATION\tTRANSACTION REJECTED: %v", err)
		return false
	}

	// Then apply
	log.Printf("VALIDATION\tTRANSACTION ACCEPTED: Applying transaction")
	_, err := ApplyTransaction(tsx, accountStates)
	if err != nil {
		log.Printf("VALIDATION\tTRANSACTION APPLICATION FAILED: %v", err)
		return false
	}
	return true
}

// ValidateTransaction validates a transaction against current state WITHOUT applying changes
func ValidateTransaction(tsx *Transaction, accountStates map[PublicKey]*AccountState) error {

	// Validate amounts are within range
	if err := utils.ValidateAmount(tsx.Amount); err != nil {
		return fmt.Errorf("invalid transaction amount: %w", err)
	}
	if err := utils.ValidateAmount(tsx.GasLimit); err != nil {
		return fmt.Errorf("invalid transaction gas limit: %w", err)
	}
	if err := utils.ValidateAmount(tsx.GasPrice); err != nil {
		return fmt.Errorf("invalid transaction gas price: %w", err)
	}

	// Coinbase transactions - always valid (no sender validation needed)
	if tsx.From == (PublicKey{}) {
		return nil
	}

	// Basic gas validation for non-coinbase transactions
	if tsx.GasLimit == 0 {
		return fmt.Errorf("gas limit cannot be zero")
	}
	if tsx.GasPrice == 0 {
		return fmt.Errorf("gas price cannot be zero")
	}

	// Regular transactions - validate only

	// 1. Signature validation
	log.Printf("VALIDATION\tValidating transaction signature from %x", tsx.From[:8])
	hash := tsx.GetHash()
	publicKey := tsx.From[:]
	signature := tsx.Signature[:]
	if !ed25519.Verify(publicKey, hash[:], signature) {
		log.Printf("VALIDATION\tInvalid signature for transaction from %x", tsx.From[:8])
		return fmt.Errorf("invalid transaction signature")
	}
	log.Printf("VALIDATION\tValid signature for transaction from %x", tsx.From[:8])

	// 2. Check sender account exists and has sufficient balance
	fromState, exists := accountStates[tsx.From]
	if !exists {
		return fmt.Errorf("sender account does not exist: %x", tsx.From[:8])
	}

	// Calculate maximum possible gas cost (gasLimit * gasPrice)
	maxGasCost := tsx.GasLimit * tsx.GasPrice
	// Check for overflow in gas cost calculation
	if tsx.GasLimit != 0 && maxGasCost/tsx.GasLimit != tsx.GasPrice {
		return fmt.Errorf("gas cost overflow: gasLimit=%d * gasPrice=%d", tsx.GasLimit, tsx.GasPrice)
	}

	// Calculate total cost: amount + max gas cost
	total, err := utils.SafeAdd(tsx.Amount, maxGasCost)
	if err != nil {
		return fmt.Errorf("transaction total with gas cost overflow: %w", err)
	}

	// Check if this is a contract deployment (To is empty and has instructions)
	isContractDeployment := tsx.To == (PublicKey{}) && len(tsx.Instructions) > 0
	if isContractDeployment {
		// Add deployment fee to total cost
		total, err = utils.SafeAdd(total, config.ContractDeploymentFee)
		if err != nil {
			return fmt.Errorf("transaction total with deployment fee overflow: %w", err)
		}
	}

	if fromState.Balance < total {
		return fmt.Errorf("insufficient balance: has %d, needs %d (amount=%d, maxGas=%d, deployment=%d)",
			fromState.Balance, total, tsx.Amount, maxGasCost,
			func() uint64 {
				if isContractDeployment {
					return config.ContractDeploymentFee
				} else {
					return 0
				}
			}())
	}

	// 3. Nonce validation (prevent double-spend)
	if tsx.Nonce != (fromState.Nonce + 1) {
		return fmt.Errorf("invalid nonce: expected %d, got %d", fromState.Nonce+1, tsx.Nonce)
	}

	// All validation passed - transaction is valid
	return nil
}

// ApplyTransaction applies a valid transaction to account states (assumes already validated)
// Returns error and gas consumed
func ApplyTransaction(tsx *Transaction, accountStates map[PublicKey]*AccountState) (uint64, error) {
	log.Printf("APPLY\tApplying transaction: %x -> %x, amount=%d, nonce=%d",
		tsx.From[:4], tsx.To[:4], tsx.Amount, tsx.Nonce)

	// Coinbase transactions
	if tsx.From == (PublicKey{}) {
		log.Printf("APPLY\tProcessing coinbase transaction")

		// Credit the recipient
		if toState, ok := accountStates[tsx.To]; ok {
			newBalance, err := utils.SafeAdd(toState.Balance, tsx.Amount)
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
		return 0, nil // Coinbase uses no gas
	}

	// Regular transactions
	fromState, exists := accountStates[tsx.From]
	if !exists {
		return 0, fmt.Errorf("sender account %x does not exist", tsx.From[:8])
	}

	// We'll calculate actual costs as we go, starting with amount
	totalCostSoFar := tsx.Amount

	// Check if this is a contract deployment
	isContractDeployment := tsx.To == (PublicKey{}) && len(tsx.Instructions) > 0
	if isContractDeployment {
		// Add deployment fee to total cost
		var err error
		totalCostSoFar, err = utils.SafeAdd(totalCostSoFar, config.ContractDeploymentFee)
		if err != nil {
			panic(fmt.Sprintf("deployment fee overflow during apply: %v", err))
		}
	}

	// Handle contract deployment or regular transfer
	if isContractDeployment {
		// Generate contract address from sender + nonce
		contractAddr := GenerateContractAddress(tsx.From, fromState.Nonce)

		gasUsed := config.GasContractDeploy // Base gas for contract deployment

		// Add gas cost for transaction data
		dataGasCost := uint64(len(tsx.Data)) * config.GasDataPerByte
		gasUsed += dataGasCost

		// Validate initialization instructions if present
		if len(tsx.InitInstructions) > 0 {
			// Validate initialization instructions
			for _, instr := range tsx.InitInstructions {
				if err := instr.ValidateInstruction(); err != nil {
					return gasUsed, fmt.Errorf("invalid initialization instruction: %v", err)
				}
			}
		}

		// Execute contract deployment - requires executor to be available
		if contractExecutor == nil {
			return gasUsed, fmt.Errorf("contract executor not available for deployment")
		}

		// Create blockchain context for execution
		blockContext := &avm.BlockchainContext{
			BlockNumber:     1, // Default values - will be overridden by actual block context when available
			BlockHash:       Hash32{},
			LastBlockHash:   Hash32{},
			Timestamp:       1640995200, // Jan 1, 2022 - default timestamp
			Difficulty:      0x207fffff,
			GasLimit:        tsx.GasLimit,
			Coinbase:        PublicKey{},
			ChainID:         1,
			Caller:          tsx.From,
			Origin:          tsx.From,
			GasPrice:        tsx.GasPrice,
			CallValue:       tsx.Amount,
			ContractAddress: contractAddr,
			TsxData:         tsx.Data,
			Deployer:        tsx.From,
			DeploymentTime:  1640995200,
			DeploymentGas:   tsx.GasPrice,
		}

		// Convert account states to execution format
		execAccountStates := make(map[types.PublicKey]*AccountState)
		for k, v := range accountStates {
			execAccountStates[types.PublicKey(k)] = v
		}

		// Execute contract deployment with initialization
		executionGasUsed, err := contractExecutor.ExecuteTransaction(tsx, execAccountStates, blockContext)
		if err != nil {
			return gasUsed, fmt.Errorf("contract deployment execution failed: %v", err)
		}

		// Copy back execution results to blockchain account states
		for k, v := range execAccountStates {
			accountStates[PublicKey(k)] = v
		}

		// Add execution gas to base gas (base deployment + data gas + execution gas)
		gasUsed += executionGasUsed

		// Validate gas usage doesn't exceed limit (Shouldnt be possible)
		if gasUsed > tsx.GasLimit {
			return gasUsed, fmt.Errorf("gas used %d exceeds gas limit %d", gasUsed, tsx.GasLimit)
		}

		// Calculate actual gas cost and deduct from sender balance
		actualGasCost := gasUsed * tsx.GasPrice
		totalCostSoFar, err = utils.SafeAdd(totalCostSoFar, actualGasCost)
		if err != nil {
			panic(fmt.Sprintf("total cost with gas overflow during apply: %v", err))
		}

		// Deduct total cost from sender
		newFromBalance, err := utils.SafeSubtract(fromState.Balance, totalCostSoFar)
		if err != nil {
			panic(fmt.Sprintf("sender balance underflow during apply: %v", err))
		}
		fromState.Balance = newFromBalance
		fromState.Nonce += 1
		log.Printf("APPLY\tSender %x new balance=%d, nonce=%d, gasUsed=%d", tsx.From[:4], fromState.Balance, fromState.Nonce, gasUsed)

		return gasUsed, nil
	} else if tsx.To != (PublicKey{}) {
		// Regular transfer to existing or new account
		gasUsed := config.GasTransfer // Base gas for transfer

		// Add gas cost for transaction data
		dataGasCost := uint64(len(tsx.Data)) * config.GasDataPerByte
		gasUsed += dataGasCost

		if toState, ok := accountStates[tsx.To]; ok {
			// Transfer amount to recipient
			newToBalance, err := utils.SafeAdd(toState.Balance, tsx.Amount)
			if err != nil {
				panic(fmt.Sprintf("recipient balance overflow during apply: %v", err))
			}
			toState.Balance = newToBalance

			// Check if recipient is a contract (has runtime code)
			if len(toState.Instructions) > 0 {
				fmt.Printf("Recipient is a contract: %x\n", tsx.To[:])

				// Execute contract call - requires executor to be available
				if contractExecutor == nil {
					fmt.Printf("Contract executor not available for call\n")
					// Transaction still succeeds (money transferred) but no contract execution
				} else {
					// Create blockchain context for execution
					blockContext := &avm.BlockchainContext{
						BlockNumber:     1, // Default values - will be overridden by actual block context when available
						BlockHash:       Hash32{},
						LastBlockHash:   Hash32{},
						Timestamp:       1640995200, // Jan 1, 2022 - default timestamp
						Difficulty:      0x207fffff,
						GasLimit:        tsx.GasLimit,
						Coinbase:        PublicKey{},
						ChainID:         1,
						Caller:          tsx.From,
						Origin:          tsx.From,
						GasPrice:        tsx.GasPrice,
						CallValue:       tsx.Amount,
						ContractAddress: tsx.To,
						TsxData:         tsx.Data,
						Deployer:        toState.Address, // Use contract's own address as deployer
						DeploymentTime:  1640995200,
						DeploymentGas:   tsx.GasPrice,
					}

					// Convert account states to execution format
					execAccountStates := make(map[types.PublicKey]*AccountState)
					for k, v := range accountStates {
						execAccountStates[types.PublicKey(k)] = v
					}

					// Execute contract call
					contractGasUsed, err := contractExecutor.ExecuteTransaction(tsx, execAccountStates, blockContext)
					if err != nil {
						fmt.Printf("Contract call execution failed: %v\n", err)
						// Contract execution failed but transaction still succeeds (money transferred)
					} else {
						// Copy back execution results to blockchain account states
						for k, v := range execAccountStates {
							accountStates[PublicKey(k)] = v
						}
						// Add contract execution gas to total
						gasUsed += contractGasUsed
					}
				}
			}
		} else {
			// Create new account
			accountStates[tsx.To] = &AccountState{
				Balance:     tsx.Amount,
				Address:     tsx.To,
				Nonce:       0,
				Persistent:  make(map[string]string),
				StorageRoot: Hash32{},
				CodeHash:    Hash32{},
			}
			fmt.Printf("Created new account: %x\n", tsx.To[:])
		}

		// Validate gas usage doesn't exceed limit
		if gasUsed > tsx.GasLimit {
			return gasUsed, fmt.Errorf("gas used %d exceeds gas limit %d", gasUsed, tsx.GasLimit)
		}

		// Calculate actual gas cost and deduct from sender balance
		actualGasCost := gasUsed * tsx.GasPrice
		var err error
		totalCostSoFar, err = utils.SafeAdd(totalCostSoFar, actualGasCost)
		if err != nil {
			panic(fmt.Sprintf("total cost with gas overflow during apply: %v", err))
		}

		// Deduct total cost from sender
		newFromBalance, err := utils.SafeSubtract(fromState.Balance, totalCostSoFar)
		if err != nil {
			panic(fmt.Sprintf("sender balance underflow during apply: %v", err))
		}
		fromState.Balance = newFromBalance
		fromState.Nonce += 1
		log.Printf("APPLY\tSender %x new balance=%d, nonce=%d, gasUsed=%d", tsx.From[:4], fromState.Balance, fromState.Nonce, gasUsed)

		return gasUsed, nil
	}
	return config.GasTransfer, nil // Base gas for regular transfer
}

// ValidateBlock validates a block and its transactions, returning the new account states if valid
func ValidateBlock(block *Block, chain *Chain) (map[PublicKey]*AccountState, error) {
	// First validate block structure (PoW, hashes, etc.)
	if err := validateBlockStructure(block, chain); err != nil {
		// Propagate ErrSwitchChain and ErrMissingParent directly, wrap other errors
		if _, ok := err.(ErrSwitchChain); ok {
			return nil, err
		}
		if _, ok := err.(ErrMissingParent); ok {
			return nil, err
		}
		return nil, fmt.Errorf("block structure validation failed: %w", err)
	}

	// Create a copy of account states for validation without modifying original
	accountStatesCopy := make(map[PublicKey]*AccountState)
	for k, v := range chain.AccountStates {
		accountStatesCopy[k] = &AccountState{
			Address:      v.Address,
			Balance:      v.Balance,
			Nonce:        v.Nonce,
			Instructions: v.Instructions,
			Persistent:   make(map[string]string),
			StorageRoot:  v.StorageRoot,
			CodeHash:     v.CodeHash,
		}
		// Copy persistent storage
		for pk, pv := range v.Persistent {
			accountStatesCopy[k].Persistent[pk] = pv
		}
	}

	// Validate and execute each transaction, updating the account states copy
	var totalGasFees uint64 = 0
	var totalGasUsed uint64 = 0

	for i, tsx := range block.Transactions {
		// First validate the transaction
		if err := ValidateTransaction(&tsx, accountStatesCopy); err != nil {
			return nil, fmt.Errorf("transaction %d failed validation: %w", i, err)
		}

		// Then apply the transaction to get actual gas usage
		gasUsed, err := ApplyTransaction(&tsx, accountStatesCopy)
		if err != nil {
			return nil, fmt.Errorf("transaction %d failed execution: %w", i, err)
		}

		// Track total gas used (skip coinbase transactions)
		if tsx.From != (PublicKey{}) {
			totalGasUsed += gasUsed
			totalGasFees += gasUsed * tsx.GasPrice
		}
	}

	// Validate that block reports correct gas usage
	if totalGasUsed != block.Header.GasUsed {
		return nil, fmt.Errorf("block reports gas used %d but actual is %d", block.Header.GasUsed, totalGasUsed)
	}

	// Validate block gas usage doesn't exceed limit
	if totalGasUsed > block.Header.GasLimit {
		return nil, fmt.Errorf("block gas used %d exceeds gas limit %d", totalGasUsed, block.Header.GasLimit)
	}

	// Validate coinbase transaction amount (skip for genesis block)
	isGenesisBlock := block.Header.GetHash() == GenesisBlock.Header.GetHash()
	if !isGenesisBlock {
		expectedCoinbaseAmount := totalGasFees + config.BlockReward
		if block.Transactions[0].Amount != expectedCoinbaseAmount {
			return nil, fmt.Errorf("coinbase transaction amount %d != expected %d",
				block.Transactions[0].Amount, expectedCoinbaseAmount)
		}
	}

	return accountStatesCopy, nil
}

// ApplyBlock applies a validated block to the chain using pre-computed account states
func ApplyBlock(block *Block, chain *Chain, newAccountStates map[PublicKey]*AccountState) error {
	// Set the new account states (already computed and validated)
	chain.SetAccountStates(newAccountStates)

	// Add the block to the chain and update tip/index
	chain.AddBlock(block)
	return nil
}

// ValidateAndApplyBlock validates block structure, applies transactions, and adds block to chain
func ValidateAndApplyBlock(block *Block, chain *Chain) error {
	// First validate and get new account states
	newAccountStates, err := ValidateBlock(block, chain)
	if err != nil {
		return err
	}

	// Then apply using the pre-computed states
	if err := ApplyBlock(block, chain, newAccountStates); err != nil {
		return fmt.Errorf("failed to apply block: %w", err)
	}
	return nil
}

// ValidateHeaderChain performs lightweight validation on a chain of headers
func ValidateHeaderChain(headers []BlockHeader) bool {
	if len(headers) == 0 {
		return false
	}

	// Basic validation - each header should link to the previous
	for i := 1; i < len(headers); i++ {
		prevHash := headers[i-1].GetHash()
		if headers[i].PreviousHash != prevHash {
			return false
		}

		// Height should increment by 1
		if headers[i].Height != headers[i-1].Height+1 {
			return false
		}

		// PoW validation - check if hash meets the target specified in header
		if err := ValidateHeaderStructure(&headers[i]); err != nil {
			log.Printf("VALIDATION\tHeader %d failed structure validation: %v", i, err)
			return false
		}

		// Timestamp validation - must be greater than previous block
		if headers[i].Timestamp <= headers[i-1].Timestamp {
			log.Printf("VALIDATION\tHeader %d timestamp not greater than previous", i)
			return false
		}
	}

	return true
}

// ValidateHeaderChainWithWork validates headers including TotalWork calculations
func ValidateHeaderChainWithWork(headers []BlockHeader, startingTotalWork string) bool {
	if len(headers) == 0 {
		return false
	}

	currentTotalWork := startingTotalWork

	for i, header := range headers {
		// Validate with parent total work
		if err := ValidateHeaderTotalWork(&header, currentTotalWork); err != nil {
			log.Printf("VALIDATION\tHeader %d failed TotalWork validation: %v", i, err)
			return false
		}

		// Validate linking to previous header (except first)
		if i > 0 {
			prevHash := headers[i-1].GetHash()
			if header.PreviousHash != prevHash {
				log.Printf("VALIDATION\tHeader %d doesn't link to previous", i)
				return false
			}

			// Height should increment by 1
			if header.Height != headers[i-1].Height+1 {
				log.Printf("VALIDATION\tHeader %d height incorrect", i)
				return false
			}

			// Timestamp validation
			if header.Timestamp <= headers[i-1].Timestamp {
				log.Printf("VALIDATION\tHeader %d timestamp not greater than previous", i)
				return false
			}
		}

		// Update current total work for next iteration
		currentTotalWork = header.TotalWork
	}

	return true
}


