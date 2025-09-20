package contracts

import (
	"august/avm"
	"august/blockchain"
	"august/execution"
	"august/types"
	"fmt"
)

// ExecuteContractAfterDeployment executes a deployed contract's initialization code
// This is for testing purposes to simulate contract execution after deployment
func ExecuteContractAfterDeployment(tsx *blockchain.Transaction, accountStates map[blockchain.PublicKey]*blockchain.AccountState, contractAddr blockchain.PublicKey) (uint64, error) {
	// Fix the sender nonce for execution - it should be the nonce BEFORE deployment
	// The blockchain deployment incremented the nonce, but execution needs the original nonce
	if fromState, exists := accountStates[tsx.From]; exists {
		fromState.Nonce-- // Temporarily decrement for execution
		defer func() {
			fromState.Nonce++ // Restore after execution
		}()
	}
	// Create a mock block for execution context
	mockBlock := &blockchain.Block{
		Header: blockchain.BlockHeader{
			Height:       1,
			Timestamp:    1640995200, // Jan 1, 2022
			Bits:         0x207fffff,
			PreviousHash: blockchain.Hash32{},
			Beneficiary:  blockchain.PublicKey{},
		},
	}

	// Create blockchain context
	blockContext := &avm.BlockchainContext{
		BlockNumber:     mockBlock.Header.Height,
		BlockHash:       mockBlock.Header.GetHash(),
		LastBlockHash:   mockBlock.Header.PreviousHash,
		Timestamp:       mockBlock.Header.Timestamp,
		Difficulty:      uint64(mockBlock.Header.Bits),
		GasLimit:        tsx.GasLimit,
		Coinbase:        mockBlock.Header.Beneficiary,
		ChainID:         1,
		Caller:          tsx.From,
		Origin:          tsx.From,
		GasPrice:        tsx.GasPrice,
		CallValue:       tsx.Amount,
		ContractAddress: contractAddr,
		Deployer:        tsx.From,
		DeploymentTime:  mockBlock.Header.Timestamp,
		DeploymentGas:   tsx.GasPrice,
	}

	// Convert account states for execution
	execAccountStates := make(map[types.PublicKey]*blockchain.AccountState)
	for k, v := range accountStates {
		execAccountStates[types.PublicKey(k)] = v
	}

	// Execute the contract
	executionGasUsed, err := execution.ExecuteTransaction(tsx, execAccountStates, blockContext)
	if err != nil {
		fmt.Printf("Contract execution failed: %v\n", err)
		return 0, err
	} else {
		fmt.Printf("Contract executed successfully, gas used: %d\n", executionGasUsed)

		// Copy back execution results
		for k, v := range execAccountStates {
			if originalState, exists := accountStates[blockchain.PublicKey(k)]; exists {
				originalState.Persistent = v.Persistent
				originalState.StorageRoot = v.StorageRoot
			}
		}
	}

	return executionGasUsed, nil
}

// ExecuteContractCall executes a call to an existing contract
// This is for testing purposes to simulate contract call execution
func ExecuteContractCall(tsx *blockchain.Transaction, accountStates map[blockchain.PublicKey]*blockchain.AccountState) (uint64, error) {
	// For a contract call, the To field contains the contract address
	contractAddr := tsx.To

	// Check if the recipient is a contract
	contractState, exists := accountStates[contractAddr]
	if !exists || len(contractState.Instructions) == 0 {
		// Not a contract, just a regular transfer
		return 0, nil
	}

	// Create a mock block for execution context
	mockBlock := &blockchain.Block{
		Header: blockchain.BlockHeader{
			Height:       1,
			Timestamp:    1640995200, // Jan 1, 2022
			Bits:         0x207fffff,
			PreviousHash: blockchain.Hash32{},
			Beneficiary:  blockchain.PublicKey{},
		},
	}

	// Create blockchain context for contract call
	blockContext := &avm.BlockchainContext{
		BlockNumber:     mockBlock.Header.Height,
		BlockHash:       mockBlock.Header.GetHash(),
		LastBlockHash:   mockBlock.Header.PreviousHash,
		Timestamp:       mockBlock.Header.Timestamp,
		Difficulty:      uint64(mockBlock.Header.Bits),
		GasLimit:        tsx.GasLimit,
		Coinbase:        mockBlock.Header.Beneficiary,
		ChainID:         1,
		Caller:          tsx.From,
		Origin:          tsx.From,
		GasPrice:        tsx.GasPrice,
		CallValue:       tsx.Amount,
		ContractAddress: contractAddr,
		Deployer:        contractState.Address, // Use the contract's own address as deployer for now
		DeploymentTime:  mockBlock.Header.Timestamp,
		DeploymentGas:   tsx.GasPrice,
	}

	// Create runtime with the contract's code
	runtime := avm.NewRuntime(tsx.GasLimit, contractState.Instructions, blockContext)

	// Copy existing persistent storage to runtime
	for key, value := range contractState.Persistent {
		runtime.Persistent[key] = value
	}

	// Execute the contract
	executionGasUsed, err := runtime.StartExecution()
	if err != nil && err != avm.ErrProgramStopped {
		fmt.Printf("Contract call execution failed: %v\n", err)
		return executionGasUsed, err
	}

	// Copy back persistent storage changes
	for key, value := range runtime.Persistent {
		contractState.Persistent[key] = value
	}

	fmt.Printf("Contract call executed successfully, gas used: %d\n", executionGasUsed)
	return executionGasUsed, nil
}