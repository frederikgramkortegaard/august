package blockchain

import (
	"august/avm"
	"august/config"
	"august/utils"
	"crypto/ed25519"
	"fmt"
	"log"
)

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

		// Create initial contract state (only stored if init succeeds)
		contractState := &AccountState{
			Balance:      tsx.Amount, // Initial balance sent to contract
			Address:      contractAddr,
			Nonce:        0,
			Instructions: tsx.Instructions, // Runtime code
			Persistent:   make(map[string]string),
			StorageRoot:  ComputeStorageRoot(nil),
			CodeHash:     ComputeCodeHash(tsx.Instructions),
		}

		// Track whether contract deployment succeeds
		contractDeployed := false

		gasUsed := config.GasContractDeploy // Base gas for contract deployment

		// Execute initialization instructions if present
		if len(tsx.InitInstructions) > 0 {
			// Check if we have enough gas remaining for init execution
			remainingGas := tsx.GasLimit - gasUsed
			if remainingGas == 0 {
				return gasUsed, fmt.Errorf("no gas remaining for contract initialization")
			}

			// Create AVM runtime for initialization using remaining gas
			runtime := avm.NewRuntime(remainingGas, tsx.InitInstructions)

			// Execute initialization code
			initGasUsed, err := runtime.StartExecution()

			// Gas is consumed regardless of success/failure
			gasUsed += initGasUsed

			// Check execution result
			if err != nil && err != avm.ErrProgramStopped {
				// Init execution failed (could be out of gas or other error)
				fmt.Printf("Contract initialization failed: %v, gas used: %d\n", err, initGasUsed)
				// Contract deployment failed - don't store it
				contractDeployed = false
			} else {
				// Apply persistent storage changes from init execution
				for key, value := range runtime.Persistent {
					contractState.Persistent[key] = value
				}
				// Update storage root to reflect persistent storage changes
				contractState.StorageRoot = ComputeStorageRoot(contractState.Persistent)
				fmt.Printf("Contract initialized successfully, gas used: %d, stored %d values\n", initGasUsed, len(runtime.Persistent))
				contractDeployed = true
			}
		} else {
			// No init instructions - deployment succeeds immediately
			contractDeployed = true
		}

		// Only store the contract if deployment succeeded
		if contractDeployed {
			accountStates[contractAddr] = contractState
			fmt.Printf("Deployed contract at: %x\n", contractAddr[:])
		} else {
			fmt.Printf("Contract deployment failed, no contract stored\n")
		}

		// Validate gas usage doesn't exceed limit (Shouldnt be possible)
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
	} else if tsx.To != (PublicKey{}) {
		// Regular transfer to existing or new account
		gasUsed := config.GasTransfer // Base gas for transfer

		if toState, ok := accountStates[tsx.To]; ok {
			// Transfer amount to recipient
			newToBalance, err := utils.SafeAdd(toState.Balance, tsx.Amount)
			if err != nil {
				panic(fmt.Sprintf("recipient balance overflow during apply: %v", err))
			}
			toState.Balance = newToBalance

			// Check if recipient is a contract (has runtime code)
			if len(toState.Instructions) > 0 {
				fmt.Printf("Executing contract at: %x\n", tsx.To[:])

				// Check if we have enough gas remaining for contract execution
				remainingGas := tsx.GasLimit - gasUsed
				if remainingGas > 0 {
					// Create AVM runtime for contract execution using remaining gas
					runtime := avm.NewRuntime(remainingGas, toState.Instructions)

					// Initialize runtime with existing persistent storage from contract
					for key, value := range toState.Persistent {
						runtime.Persistent[key] = value
					}

					// Execute contract runtime code
					runtimeGasUsed, err := runtime.StartExecution()

					// Gas is consumed regardless of success/failure
					gasUsed += runtimeGasUsed

					if err != nil && err != avm.ErrProgramStopped {
						fmt.Printf("Contract execution failed: %v, gas used: %d\n", err, runtimeGasUsed)
						// Contract execution failed but transaction still succeeds (money transferred)
						// This matches Ethereum behavior - failed contract calls still deduct gas
					} else {
						fmt.Printf("Contract executed successfully, gas used: %d\n", runtimeGasUsed)
						// Replace contract's persistent storage with runtime's final state
						// The runtime started with a copy of all existing storage, so this preserves everything
						toState.Persistent = runtime.Persistent
						// Update storage root to reflect persistent storage changes
						toState.StorageRoot = ComputeStorageRoot(toState.Persistent)
						fmt.Printf("Applied %d persistent storage updates\n", len(runtime.Persistent))
					}
				} else {
					fmt.Printf("No gas remaining for contract execution\n")
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

	// Validate each transaction without executing contracts or modifying state
	for i, tsx := range block.Transactions {
		if err := ValidateTransaction(&tsx, accountStatesCopy); err != nil {
			return fmt.Errorf("transaction %d failed validation: %w", i, err)
		}

		// For sequential transaction validation, we need to simulate state changes
		// without actually executing contracts (just update balances/nonces for validation)
		if tsx.From != (PublicKey{}) {
			fromState := accountStatesCopy[tsx.From]
			fromState.Nonce += 1

			// Deduct maximum possible cost for subsequent transaction validation
			maxGasCost := tsx.GasLimit * tsx.GasPrice
			maxCost := tsx.Amount + maxGasCost
			if tsx.To == (PublicKey{}) && len(tsx.Instructions) > 0 {
				maxCost += config.ContractDeploymentFee
			}
			fromState.Balance -= maxCost
		}
	}

	// Use the gas usage reported in block header (trust but verify until we execute the contract)
	totalGasUsed := block.Header.GasUsed

	// Validate block gas usage doesn't exceed limit
	if totalGasUsed > block.Header.GasLimit {
		return fmt.Errorf("block gas used %d exceeds gas limit %d", totalGasUsed, block.Header.GasLimit)
	}

	// For lightweight validation, we trust the miner's gas calculations
	// Exact gas fees and coinbase validation will be done during ApplyBlock
	// where we actually execute the transactions

	return nil
}

// ApplyBlock applies a validated block to the chain (assumes already validated)
func ApplyBlock(block *Block, chain *Chain) error {
	var totalGasFees uint64 = 0
	var totalGasUsed uint64 = 0

	// Apply each transaction to the chain state and track actual gas usage
	for i, tsx := range block.Transactions {
		gasUsed, err := ApplyTransaction(&tsx, chain.AccountStates)
		if err != nil {
			return fmt.Errorf("failed to apply transaction %d: %w", i, err)
		}

		// Debug logging
		fmt.Printf("VALIDATION DEBUG: tx %d, gasUsed=%d, isCoinbase=%t, from=%x\n",
			i, gasUsed, tsx.From == (PublicKey{}), tsx.From[:8])

		// Track total gas used (skip coinbase transactions like mining does)
		if tsx.From != (PublicKey{}) {
			newGasUsed, err := utils.SafeAdd(totalGasUsed, gasUsed)
			if err != nil {
				return fmt.Errorf("gas used sum overflow: %w", err)
			}
			totalGasUsed = newGasUsed

			// Calculate gas fees for non-coinbase transactions
			gasFee := gasUsed * tsx.GasPrice
			newGasFees, err := utils.SafeAdd(totalGasFees, gasFee)
			if err != nil {
				return fmt.Errorf("gas fees sum overflow: %w", err)
			}
			totalGasFees = newGasFees
		}
	}

	// Debug total
	fmt.Printf("VALIDATION DEBUG: block reports %d gas, validation calculated %d gas\n",
		block.Header.GasUsed, totalGasUsed)

	// Validate that block reports correct gas usage
	if totalGasUsed != block.Header.GasUsed {
		return fmt.Errorf("block reports gas used %d but actual is %d", block.Header.GasUsed, totalGasUsed)
	}

	// Skip coinbase validation for genesis block (handled in structure validation)
	isGenesisBlock := block.Header.GetHash() == GenesisBlock.Header.GetHash()
	if !isGenesisBlock {
		// Validate coinbase transaction amount based on actual gas fees
		expectedCoinbaseAmount, err := utils.SafeAdd(totalGasFees, config.BlockReward)
		if err != nil {
			return fmt.Errorf("coinbase amount calculation overflow: %w", err)
		}
		if block.Transactions[0].Amount != expectedCoinbaseAmount {
			return fmt.Errorf("coinbase transaction amount %d != expected %d (gas fees %d + block reward %d)",
				block.Transactions[0].Amount, expectedCoinbaseAmount, totalGasFees, config.BlockReward)
		}
	}

	// Add the block to the chain and update tip/index
	chain.AddBlock(block)
	return nil
}

// ValidateAndApplyBlock validates block structure, applies transactions, and adds block to chain
func ValidateAndApplyBlock(block *Block, chain *Chain) error {
	// First validate
	if err := ValidateBlock(block, chain); err != nil {
		return err
	}

	// Then apply
	if err := ApplyBlock(block, chain); err != nil {
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


