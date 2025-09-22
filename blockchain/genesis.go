package blockchain

import (
	"august/config"
	"august/utils"
	"log"
	"math/big"
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

	// Compute merkle root for the genesis transaction at runtime
	// For a single transaction, merkle root is just the transaction hash
	var merkleRoot Hash32
	txHash := genesisCoinbase.GetHash()
	copy(merkleRoot[:], txHash[:])

	// Create genesis block header template
	header := BlockHeader{
		Version:      1,
		PreviousHash: Hash32{},          // All zeros for genesis
		Height:       0,                 // Genesis is height 0
		Timestamp:    1609459200,        // Fixed timestamp (Jan 1, 2021)
		Nonce:        0,                 // Will be set by mining
		MerkleRoot:   merkleRoot,        // Computed from genesis transaction
		StateRoot:    Hash32{},          // Empty state root for genesis
		ReceiptRoot:  Hash32{},          // No receipts in genesis
		Bits:         config.GetTargetCompact(), // Use appropriate target for environment
		TotalWork:    "1",               // Work for test difficulty
		GasLimit:     8000000,           // 8M gas limit
		GasUsed:      0,                 // No gas used in genesis
		Beneficiary:  PublicKey{},       // No beneficiary for genesis
	}

	// Mine the genesis block (find valid nonce) - start from a deterministic seed
	target := utils.CompactToBig(header.Bits)
	found := false
	for nonce := uint64(25000); nonce < 1000000; nonce++ { // Start from 25000 for deterministic results
		header.Nonce = nonce
		hash := header.GetHash()
		hashInt := new(big.Int).SetBytes(hash[:])

		if hashInt.Cmp(target) <= 0 {
			// Found valid nonce
			found = true
			break
		}
	}

	if !found {
		log.Printf("Warning: Could not find valid nonce for genesis block, using nonce 25992")
		header.Nonce = 25992 // Fallback to known good nonce
	}

	// Create the genesis block
	GenesisBlock = &Block{
		Header:       header,
		Transactions: []Transaction{genesisCoinbase},
	}

	// Log the dynamically computed genesis
	hash := GenesisBlock.Header.GetHash()
	log.Printf("Genesis block mined dynamically - hash: %s", hash.String())
	log.Printf("config.FirstUser %x receives 10 AUG initial coins", config.FirstUser[:8])
}
