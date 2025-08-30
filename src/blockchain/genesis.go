package blockchain

import (
	"gocuria/src/config"
	"gocuria/src/hashing"
	"gocuria/src/mining"
	"gocuria/src/models"
	"log"
)

// GenesisBlock is the first block in the blockchain
// It has no previous hash and contains no real transactions
var GenesisBlock *models.Block

func init() {
	// Create genesis coinbase transaction - initial coin supply
	genesisCoinbase := models.Transaction{
		From:      models.PublicKey{}, // Coinbase (empty)
		To:        config.FirstUser,   // Send to FirstUser
		Amount:    10000000,           // 10 million initial coins
		Nonce:     0,                  // Coinbase doesn't need nonce
		Signature: models.Signature{}, // Coinbase doesn't need signature
	}

	// Calculate merkle root for the single transaction
	merkleRoot := hashing.MerkleTransactions([]models.Transaction{genesisCoinbase})

	// Create genesis block header
	header := models.BlockHeader{
		Version:      1,
		PreviousHash: [32]byte{}, // All zeros for genesis
		Timestamp:    0,
		Nonce:        0,
		MerkleRoot:   merkleRoot, // Merkle root of genesis transaction
	}

	// Mine the genesis block
	nonce, err := mining.MineCorrectNonce(&header)
	if err != nil {
		log.Fatal("Failed to mine genesis block:", err)
	}
	header.Nonce = nonce

	// Create the genesis block with the coinbase transaction
	GenesisBlock = &models.Block{
		Header:       header,
		Transactions: []models.Transaction{genesisCoinbase},
	}

	// Log the genesis block hash for reference
	hash := hashing.HashBlockHeader(&header)
	log.Printf("Genesis block created with nonce %d, hash: %x\n", nonce, hash[:])
	log.Printf("FirstUser %x receives 10M initial coins\n", config.FirstUser[:8])
}
