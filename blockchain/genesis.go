package blockchain

import (
	"august/config"
	"log"
)

// GenesisBlock is the first block in the blockchain
// It has no previous hash and contains no real transactions
var GenesisBlock *Block

func init() {
	// Hardcoded genesis block - pre-computed to avoid any runtime calculation
	// Genesis coinbase transaction gives 10M coins to config.FirstUser
	genesisCoinbase := Transaction{
		From:             PublicKey{}, // Coinbase (empty)
		To:               config.FirstUser,   // Send to config.FirstUser
		Amount:           10 * config.AUG,    // 10 AUG initial coins
		Nonce:            0,           // Coinbase doesn't need nonce
		Signature:        Signature{}, // Coinbase doesn't need signature
		Timestamp:        1704067200,  // Fixed timestamp for genesis (Jan 1, 2024)
		ChainID:          1,           // Main network chain ID
		Instructions:     nil,         // No contract code
		InitInstructions: nil,         // No init code
		GasLimit:         0,           // Coinbase doesn't consume gas
		GasPrice:         0,           // Coinbase doesn't pay gas
	}

	// Pre-computed genesis block header (properly mined with valid nonce and merkle root)
	header := BlockHeader{
		Version:      1,
		PreviousHash: Hash32{},                                                                                                                                                                                               // All zeros for genesis
		Height:       0,                                                                                                                                                                                                      // Genesis is height 0
		Timestamp:    1609459200,                                                                                                                                                                                             // Fixed timestamp (Jan 1, 2021)
		Nonce:        1944729,                                                                                                                                                                                                // Properly mined valid nonce
		MerkleRoot:   Hash32{0xc6, 0xa0, 0xbb, 0x4e, 0xea, 0xe3, 0x33, 0xa9, 0x82, 0xc7, 0x0c, 0xe1, 0x15, 0x44, 0x0b, 0x89, 0xdb, 0x8f, 0x6e, 0x2e, 0x36, 0x19, 0x3e, 0x65, 0x7f, 0xa9, 0x57, 0x68, 0xee, 0xa3, 0x30, 0x00}, // Actual computed merkle root
		StateRoot:    Hash32{},          // Empty state root for genesis
		ReceiptRoot:  Hash32{},          // No receipts in genesis
		Bits:         config.TestTargetCompact, // Super easy target for tests
		TotalWork:    "1048578",         // Computed work value
		GasLimit:     8000000,           // 8M gas limit
		GasUsed:      0,                 // No gas used in genesis
		Beneficiary:  PublicKey{},       // No beneficiary for genesis
	}

	// Create the hardcoded genesis block
	GenesisBlock = &Block{
		Header:       header,
		Transactions: []Transaction{genesisCoinbase},
	}

	log.Printf("Genesis block loaded (hardcoded) - hash: 00000db1b197dba918877f28230252126e7f2fd4e883941f1be04961e8504b35")
	log.Printf("config.FirstUser %x receives 10 AUG initial coins\n", config.FirstUser[:8])
}
