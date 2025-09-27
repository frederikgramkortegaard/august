package blockchain

import (
	"august/config"
	"august/utils"
	"errors"
	"fmt"
	"time"
)

type NonceType = uint64

type BlockCreationParams struct {
	Version       uint64
	PreviousHash  [32]byte
	Height        uint64
	PreviousWork  string // Total work of previous block as decimal string
	Coinbase      Transaction
	Transactions  []Transaction
	Timestamp     uint64
	TargetBits    uint32                      // Target in compact format
	CurrentStates map[PublicKey]*AccountState // Current blockchain state for execution
}

// NewBlock creates a new block and mines it to satisfy targetBits
func NewBlock(params BlockCreationParams) (Block, error) {
	ts := params.Timestamp
	if ts == 0 {
		ts = uint64(time.Now().Unix())
	}

	// Combine coinbase and regular transactions
	tsxs := append([]Transaction{params.Coinbase}, params.Transactions...)

	// Actually execute transactions to get exact gas usage and fees
	// Create a copy of current chain state for execution
	tempAccountStates := make(map[PublicKey]*AccountState)
	for pubKey, state := range params.CurrentStates {
		if state != nil {
			tempAccountStates[pubKey] = &AccountState{
				Address:      state.Address,
				Balance:      state.Balance,
				Nonce:        state.Nonce,
				Instructions: state.Instructions,
				Persistent:   make(map[string]string),
				StorageRoot:  state.StorageRoot,
				CodeHash:     state.CodeHash,
			}
			// Copy persistent storage
			for k, v := range state.Persistent {
				tempAccountStates[pubKey].Persistent[k] = v
			}
		}
	}

	var gasFeeSum uint64
	var totalGasUsed uint64

	for i, tx := range params.Transactions {
		// Skip coinbase transactions
		if tx.From == (PublicKey{}) {
			fmt.Printf("MINING DEBUG: tx %d, skipping coinbase, from=%x\n", i, tx.From[:8])
			continue
		}

		// Execute transaction to get actual gas usage
		gasUsed, err := ApplyTransaction(&tx, tempAccountStates)
		if err != nil {
			// Transaction failed - skip it (in practice, miners might still include failed txs that pay gas)
			fmt.Printf("MINING DEBUG: tx %d, failed with error: %v\n", i, err)
			continue
		}

		fmt.Printf("MINING DEBUG: tx %d, gasUsed=%d, from=%x\n", i, gasUsed, tx.From[:8])

		// Calculate actual gas fee
		gasFee := gasUsed * tx.GasPrice
		newGasFeeSum, err := utils.SafeAdd(gasFeeSum, gasFee)
		if err != nil {
			return Block{}, fmt.Errorf("gas fees sum overflow: %w", err)
		}
		gasFeeSum = newGasFeeSum

		newTotalGasUsed, err := utils.SafeAdd(totalGasUsed, gasUsed)
		if err != nil {
			return Block{}, fmt.Errorf("total gas used overflow: %w", err)
		}
		totalGasUsed = newTotalGasUsed
	}

	// Debug total
	fmt.Printf("MINING DEBUG: total gas used in mining: %d\n", totalGasUsed)

	// Validate Coinbase amount
	expectedCoinbaseAmount, err := utils.SafeAdd(config.BlockReward, gasFeeSum)
	if err != nil {
		return Block{}, fmt.Errorf("coinbase amount calculation overflow: %w", err)
	}
	if params.Coinbase.Amount != expectedCoinbaseAmount {
		return Block{}, fmt.Errorf("Coinbase amount (%d) must equal BlockReward (%d) + gas fees (%d)",
			params.Coinbase.Amount, config.BlockReward, gasFeeSum)
	}

	// Calculate Merkle root
	merkleRoot := MerkleTransactions(tsxs)

	// Calculate this block's work
	blockWork := GetBlockWork(params.TargetBits)

	// Total work = previous work + this block's work
	totalWork := AddWork(params.PreviousWork, blockWork.String())

	// Set block gas limit (in practice this would be a protocol parameter)
	blockGasLimit := uint64(8000000) // 8M gas per block (similar to Ethereum)
	if totalGasUsed > blockGasLimit {
		return Block{}, fmt.Errorf("transactions exceed block gas limit: used %d, limit %d", totalGasUsed, blockGasLimit)
	}

	// Initialize block header
	header := BlockHeader{
		Version:      params.Version,
		PreviousHash: params.PreviousHash,
		Height:       params.Height,
		Timestamp:    ts,
		Nonce:        0,
		MerkleRoot:   merkleRoot,
		StateRoot:    Hash32{}, // TODO: Calculate proper state root
		ReceiptRoot:  Hash32{}, // TODO: Calculate proper receipt root
		Bits:         params.TargetBits,
		TotalWork:    totalWork,
		GasLimit:     blockGasLimit,
		GasUsed:      totalGasUsed, // Actual gas used from execution
		Beneficiary:  params.Coinbase.To,
	}

	// Mine the block to find a valid nonce
	nonce, err := MineCorrectNonce(&header, params.TargetBits)
	if err != nil {
		return Block{}, err
	}
	header.Nonce = nonce

	// Construct the final block
	block := Block{
		Header:       header,
		Transactions: tsxs,
	}

	return block, nil
}

// MineCorrectNonce finds a nonce such that the block hash meets the target
func MineCorrectNonce(header *BlockHeader, targetBits uint32) (NonceType, error) {
	for {
		hash := header.GetHash()
		if BlockHashMeetsDifficulty(hash, targetBits) {
			return header.Nonce, nil
		}
		header.Nonce++

		// Avoid infinite loop if nonce overflows
		if header.Nonce == ^uint64(0) {
			return 0, errors.New("nonce overflow, could not mine block")
		}
	}
}
