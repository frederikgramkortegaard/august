package blockchain

import (
	"errors"
	"fmt"
	"time"
)

type NonceType = uint64

const (
	BlockReward     = 50     // Base block reward for mining
	HalvingInterval = 210000 // Blocks between reward halvings (optional for future)
)

type BlockCreationParams struct {
	Version      uint64
	PreviousHash [32]byte
	Height       uint64
	PreviousWork string // Total work of previous block (big.Int as string)
	Coinbase     Transaction
	Transactions []Transaction
	Timestamp    uint64
	TargetBits   uint32 // Target in compact format (replaces Difficulty)
}

func NewBlock(params BlockCreationParams) (Block, error) {
	ts := params.Timestamp
	if ts == 0 {
		ts = uint64(time.Now().Unix())
	}

	// Combine coinbase and regular transactions
	tsxs := make([]Transaction, 0, len(params.Transactions)+1)
	tsxs = append(tsxs, params.Coinbase)
	tsxs = append(tsxs, params.Transactions...)

	// Calculate transaction fees
	var tsxfeesum uint64 = 0
	for _, tsx := range params.Transactions {
		tsxfeesum += tsx.Fee
	}

	// Validate Coinbase transaction amount is BlockReward + Fees
	if params.Coinbase.Amount != tsxfeesum+BlockReward {
		return Block{}, fmt.Errorf("Coinbase transaction is not Transaction Fee's + BlockReward")
	}

	// Calculate merkle root
	merkle := MerkleTransactions(tsxs)

	// Calculate work for this block using Bitcoin-style calculation
	blockWork := GetBlockWork(params.TargetBits)

	// Calculate total work (previous work + this block's work)
	totalWork := AddWork(params.PreviousWork, blockWork.String())

	// Create header
	header := BlockHeader{
		Version:      params.Version,
		PreviousHash: params.PreviousHash,
		Height:       params.Height,
		Timestamp:    ts,
		Nonce:        0,
		MerkleRoot:   merkle,
		Bits:         params.TargetBits,
		TotalWork:    totalWork,
	}

	// Mine the header to find valid nonce
	nonce, err := MineCorrectNonce(&header, params.TargetBits)
	if err != nil {
		return Block{}, errors.New("could not find a valid nonce")
	}
	header.Nonce = nonce

	// Create and return the block
	block := Block{
		Header:       header,
		Transactions: tsxs,
	}

	return block, nil
}

func MineCorrectNonce(header *BlockHeader, targetBits uint32) (NonceType, error) {
	// Mine until we find a hash that meets the target
	for {
		hash := HashBlockHeader(header)
		if BlockHashMeetsDifficulty(hash, targetBits) {
			return header.Nonce, nil
		}
		header.Nonce++
		
		// Prevent infinite loop in case of error
		if header.Nonce == ^uint64(0) {
			return 0, errors.New("nonce overflow")
		}
	}
}