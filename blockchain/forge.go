package blockchain

import (
	"errors"
	"time"
)

type BlockCreationParams struct {
	Version      uint64
	PreviousHash [32]byte
	Coinbase     Transaction
	Transactions []Transaction
	Timestamp    uint64
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

	// Calculate merkle root
	merkle := MerkleTransactions(tsxs)

	// Create header
	header := BlockHeader{
		Version:      params.Version,
		PreviousHash: params.PreviousHash,
		Timestamp:    ts,
		Nonce:        0,
		MerkleRoot:   merkle,
	}

	// Mine the header to find valid nonce
	nonce, err := MineCorrectNonce(&header)
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
