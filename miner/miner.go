package miner

import (
	"august/blockchain"
	"errors"
	"fmt"
	"time"
)

type NonceType = uint64

type BlockCreationParams struct {
	Version      uint64
	PreviousHash [32]byte
	Height       uint64
	PreviousWork string // Total work of previous block as decimal string
	Coinbase     blockchain.Transaction
	Transactions []blockchain.Transaction
	Timestamp    uint64
	TargetBits   uint32 // Target in compact format
}

// NewBlock creates a new block and mines it to satisfy targetBits
func NewBlock(params BlockCreationParams) (blockchain.Block, error) {
	ts := params.Timestamp
	if ts == 0 {
		ts = uint64(time.Now().Unix())
	}

	// Combine coinbase and regular transactions
	tsxs := append([]blockchain.Transaction{params.Coinbase}, params.Transactions...)

	// Calculate transaction fees
	var feeSum uint64
	for _, tx := range params.Transactions {
		feeSum += tx.Fee
	}

	// Validate Coinbase amount
	if params.Coinbase.Amount != blockchain.BlockReward+feeSum {
		return blockchain.Block{}, fmt.Errorf("Coinbase amount (%d) must equal BlockReward (%d) + transaction fees (%d)",
			params.Coinbase.Amount, blockchain.BlockReward, feeSum)
	}

	// Calculate Merkle root
	merkleRoot := blockchain.MerkleTransactions(tsxs)

	// Calculate this block's work
	blockWork := blockchain.GetBlockWork(params.TargetBits)

	// Total work = previous work + this block's work
	totalWork := blockchain.AddWork(params.PreviousWork, blockWork.String())

	// Initialize block header
	header := blockchain.BlockHeader{
		Version:      params.Version,
		PreviousHash: params.PreviousHash,
		Height:       params.Height,
		Timestamp:    ts,
		Nonce:        0,
		MerkleRoot:   merkleRoot,
		Bits:         params.TargetBits,
		TotalWork:    totalWork,
	}

	// Mine the block to find a valid nonce
	nonce, err := MineCorrectNonce(&header, params.TargetBits)
	if err != nil {
		return blockchain.Block{}, err
	}
	header.Nonce = nonce

	// Construct the final block
	block := blockchain.Block{
		Header:       header,
		Transactions: tsxs,
	}

	return block, nil
}

// MineCorrectNonce finds a nonce such that the block hash meets the target
func MineCorrectNonce(header *blockchain.BlockHeader, targetBits uint32) (NonceType, error) {
	for {
		hash := blockchain.HashBlockHeader(header)
		if blockchain.BlockHashMeetsDifficulty(hash, targetBits) {
			return header.Nonce, nil
		}
		header.Nonce++

		// Avoid infinite loop if nonce overflows
		if header.Nonce == ^uint64(0) {
			return 0, errors.New("nonce overflow, could not mine block")
		}
	}
}
