package blockchain

import (
	"log"
)

// GenesisBlock is the first block in the blockchain
// It has no previous hash and contains no real transactions
var GenesisBlock *Block

func init() {
	// Create genesis coinbase transaction - initial coin supply
	genesisCoinbase := Transaction{
		From:      PublicKey{}, // Coinbase (empty)
		To:        FirstUser,   // Send to FirstUser
		Amount:    10000000,    // 10 million initial coins
		Nonce:     0,           // Coinbase doesn't need nonce
		Signature: Signature{}, // Coinbase doesn't need signature
	}

	// Calculate merkle root for the single transaction
	merkleRoot := MerkleTransactions([]Transaction{genesisCoinbase})

	// Calculate work for genesis block (difficulty = 1)
	genesisWork := CalculateBlockWork(1)

	// Create genesis block header
	header := BlockHeader{
		Version:      1,
		PreviousHash: Hash32{}, // All zeros for genesis
		Height:       0,        // Genesis is height 0
		Timestamp:    0,
		Nonce:        0,
		MerkleRoot:   merkleRoot, // Merkle root of genesis transaction
		TotalWork:    genesisWork.String(), // Proper work calculation
	}

	// Mine the genesis block
	nonce, err := MineCorrectNonce(&header, 1)
	if err != nil {
		log.Fatal("Failed to mine genesis block:", err)
	}
	header.Nonce = nonce

	// Create the genesis block with the coinbase transaction
	GenesisBlock = &Block{
		Header:       header,
		Transactions: []Transaction{genesisCoinbase},
	}

	// Log the genesis block hash for reference
	hash := HashBlockHeader(&header)
	log.Printf("Genesis block created with nonce %d, hash: %x\n", nonce, hash[:])
	log.Printf("FirstUser %x receives 10M initial coins\n", FirstUser[:8])
}
