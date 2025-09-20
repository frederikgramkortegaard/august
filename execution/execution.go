package execution

import (
	"august/avm"
	"august/blockchain"
	"august/types"
	"fmt"
)

// ExecutorImpl implements the blockchain.ContractExecutor interface
type ExecutorImpl struct{}

// init registers the executor with the blockchain package
func init() {
	blockchain.SetContractExecutor(&ExecutorImpl{})
}

// ExecuteTransaction executes a transaction including contract deployment and execution
func (e *ExecutorImpl) ExecuteTransaction(tsx *blockchain.Transaction, accountStates map[types.PublicKey]*blockchain.AccountState, blockContext *avm.BlockchainContext) (uint64, error) {
	// Determine transaction type based on content
	isContractDeployment := len(tsx.Instructions) > 0

	if isContractDeployment {
		// Handle contract deployment
		return executeContractDeployment(tsx, accountStates, blockContext)
	} else {
		// Handle regular transfer or contract call
		return executeContractCall(tsx, accountStates, blockContext)
	}
}

// executeContractDeployment handles contract deployment with initialization
func executeContractDeployment(tsx *blockchain.Transaction, accountStates map[types.PublicKey]*blockchain.AccountState, blockContext *avm.BlockchainContext) (uint64, error) {
	gasUsed := uint64(0)

	// Generate contract address
	fromState := accountStates[tsx.From]
	contractAddr := blockchain.GenerateContractAddress(tsx.From, fromState.Nonce)

	// Create contract state
	contractState := &blockchain.AccountState{
		Balance:      0,
		Address:      contractAddr,
		Nonce:        0,
		Instructions: tsx.Instructions,
		Persistent:   make(map[string]string),
		StorageRoot:  blockchain.Hash32{},
		CodeHash:     blockchain.ComputeCodeHash(tsx.Instructions),
	}

	// Execute initialization instructions if present
	if len(tsx.InitInstructions) > 0 {
		// Check if we have enough gas remaining for init execution
		remainingGas := tsx.GasLimit - gasUsed
		if remainingGas == 0 {
			return gasUsed, fmt.Errorf("no gas remaining for contract initialization")
		}

		// Create AVM runtime for initialization using remaining gas
		runtime := avm.NewRuntime(remainingGas, tsx.InitInstructions, blockContext)

		// Execute initialization code
		initGasUsed, err := runtime.StartExecution()

		// Gas is consumed regardless of success/failure
		gasUsed += initGasUsed

		// Check execution result
		if err != nil && err != avm.ErrProgramStopped {
			return gasUsed, fmt.Errorf("contract initialization failed: %v", err)
		}

		// Apply persistent storage changes from init execution
		for key, value := range runtime.Persistent {
			contractState.Persistent[key] = value
		}
		// Update storage root to reflect persistent storage changes
		contractState.StorageRoot = blockchain.ComputeStorageRoot(contractState.Persistent)
		fmt.Printf("Contract initialized successfully, gas used: %d, stored %d values\n", initGasUsed, len(runtime.Persistent))
	}

	// Store the contract
	accountStates[contractAddr] = contractState

	return gasUsed, nil
}

// executeContractCall handles calls to existing contracts
func executeContractCall(tsx *blockchain.Transaction, accountStates map[types.PublicKey]*blockchain.AccountState, blockContext *avm.BlockchainContext) (uint64, error) {
	gasUsed := uint64(0)

	// Get recipient state
	toState, exists := accountStates[tsx.To]
	if !exists {
		return gasUsed, fmt.Errorf("recipient account does not exist")
	}

	// Transfer amount to recipient
	toState.Balance += tsx.Amount

	// Check if recipient is a contract (has runtime code)
	if len(toState.Instructions) > 0 {
		fmt.Printf("Executing contract at: %x\n", tsx.To[:])

		// Check if we have enough gas remaining for contract execution
		remainingGas := tsx.GasLimit - gasUsed
		if remainingGas > 0 {
			// Create AVM runtime for contract execution using remaining gas
			runtime := avm.NewRuntime(remainingGas, toState.Instructions, blockContext)

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
			} else {
				fmt.Printf("Contract executed successfully, gas used: %d\n", runtimeGasUsed)
				// Replace contract's persistent storage with runtime's final state
				toState.Persistent = runtime.Persistent
				// Update storage root to reflect persistent storage changes
				toState.StorageRoot = blockchain.ComputeStorageRoot(toState.Persistent)
				fmt.Printf("Applied %d persistent storage updates\n", len(runtime.Persistent))
			}
		} else {
			fmt.Printf("No gas remaining for contract execution\n")
		}
	}

	return gasUsed, nil
}