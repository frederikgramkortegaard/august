package blockchain

import (
	"log"
)

// GenesisBlock is the first block in the blockchain
// It has no previous hash and contains no real transactions
var GenesisBlock *Block

func init() {
	// Hardcoded genesis block - pre-computed to avoid any runtime calculation
	// Genesis coinbase transaction gives 10M coins to FirstUser
	genesisCoinbase := Transaction{
		From:      PublicKey{}, // Coinbase (empty)
		To:        FirstUser,   // Send to FirstUser  
		Amount:    10000000,    // 10 million initial coins
		Fee:       0,           // No fee for coinbase
		Nonce:     0,           // Coinbase doesn't need nonce
		Signature: Signature{}, // Coinbase doesn't need signature
	}

	// Pre-computed genesis block header (properly mined with valid nonce and merkle root)
	header := BlockHeader{
		Version:      1,
		PreviousHash: Hash32{}, // All zeros for genesis
		Height:       0,        // Genesis is height 0
		Timestamp:    1609459200, // Fixed timestamp (Jan 1, 2021)
		Nonce:        1944729,    // Properly mined valid nonce
		MerkleRoot:   Hash32{0xc6, 0xa0, 0xbb, 0x4e, 0xea, 0xe3, 0x33, 0xa9, 0x82, 0xc7, 0x0c, 0xe1, 0x15, 0x44, 0x0b, 0x89, 0xdb, 0x8f, 0x6e, 0x2e, 0x36, 0x19, 0x3e, 0x65, 0x7f, 0xa9, 0x57, 0x68, 0xee, 0xa3, 0x30, 0x00}, // Actual computed merkle root
		Bits:         TestTargetCompact, // Easy target for tests
		TotalWork:    "1048578",         // Computed work value
	}

	// Create the hardcoded genesis block
	GenesisBlock = &Block{
		Header:       header,
		Transactions: []Transaction{genesisCoinbase},
	}

	log.Printf("Genesis block loaded (hardcoded) - hash: 00000db1b197dba918877f28230252126e7f2fd4e883941f1be04961e8504b35")
	log.Printf("FirstUser %x receives 10M initial coins\n", FirstUser[:8])
}
