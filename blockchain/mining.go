package blockchain

import (
	"errors"
	"fmt"
	"time"
)

type NonceType = uint64

const (
	MaxTarget       = 0x00000000FFFF0000000000000000000000000000000000000000000000000000
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
	Difficulty   uint64
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

	var tsxfeesum uint64 = 0
	// Then validate and apply each transaction incrementally
	for _, tsx := range params.Transactions {
		tsxfeesum += tsx.Fee
	}

	// Validate Coinbase tsx amount is Fee's + Block Reward
	if params.Coinbase.Amount != tsxfeesum+BlockReward {
		return Block{}, fmt.Errorf("Coinbase transaction is not Transaction Fee's + BlockReward")
	}
	// Calculate merkle root
	merkle := MerkleTransactions(tsxs)

	// Calculate work for this block
	blockWork := CalculateBlockWork(params.Difficulty)

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
		TotalWork:    totalWork,
	}

	// Mine the header to find valid nonce
	nonce, err := MineCorrectNonce(&header, params.Difficulty)
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

func MineCorrectNonce(header *BlockHeader, difficulty uint64) (NonceType, error) {

	for hash := HashBlockHeader(header); !BlockHashMeetsDifficulty(hash, difficulty); hash = HashBlockHeader(header) {
		header.Nonce += 1
	}

	return header.Nonce, nil

}
