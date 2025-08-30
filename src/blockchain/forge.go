package blockchain

import (
	"errors"
	"gocuria/src/hashing"
	"gocuria/src/mining"
	"gocuria/src/models"
	"time"
)

type BlockCreationParams struct {
	Version      uint64
	PreviousHash [32]byte
	Coinbase     models.Transaction
	Transactions []models.Transaction
	Timestamp    uint64
}

func NewBlock(params BlockCreationParams) (models.Block, error) {
	ts := params.Timestamp
	if ts == 0 {
		ts = uint64(time.Now().Unix())
	}

	// Combine coinbase and regular transactions
	tsxs := make([]models.Transaction, 0, len(params.Transactions)+1)
	tsxs = append(tsxs, params.Coinbase)
	tsxs = append(tsxs, params.Transactions...)

	// Calculate merkle root
	merkle := hashing.MerkleTransactions(tsxs)

	// Create header
	header := models.BlockHeader{
		Version:      params.Version,
		PreviousHash: params.PreviousHash,
		Timestamp:    ts,
		Nonce:        0,
		MerkleRoot:   merkle,
	}

	// Mine the header to find valid nonce
	nonce, err := mining.MineCorrectNonce(&header)
	if err != nil {
		return models.Block{}, errors.New("could not find a valid nonce")
	}
	header.Nonce = nonce

	// Create and return the block
	block := models.Block{
		Header:       header,
		Transactions: tsxs,
	}

	return block, nil
}
