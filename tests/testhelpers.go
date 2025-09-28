package tests

import (
	"august/blockchain"
	"august/config"
	"august/testutils"
	"time"
)

// CreateTestBlockWithCoinbase creates a block with coinbase transaction
// This is what most tests will want - a valid block with proper coinbase
func CreateTestBlockWithCoinbase(parent *blockchain.BlockHeader, beneficiary blockchain.PublicKey, additionalTxs []*blockchain.Transaction) *blockchain.Block {

	// Create coinbase transaction
	coinbase := &blockchain.Transaction{
		From:             blockchain.PublicKey{}, // Zero hash for coinbase
		To:               beneficiary,
		Amount:           config.BlockReward, // 50 AUG block reward
		Nonce:            0,
		Timestamp:        uint64(time.Now().UnixNano()), // Use actual time for realistic timestamps
		ChainID:          config.MainnetChainID,
		Instructions:     []blockchain.Instruction{},
		GasLimit:         0, // Coinbase doesn't consume gas
		GasPrice:         0,
		Data:             []byte{},
		Signature:        blockchain.Signature{}, // Coinbase doesn't need signature
	}

	// Combine coinbase with additional transactions
	allTxs := append([]*blockchain.Transaction{coinbase}, additionalTxs...)

	return testutils.CreateTestBlock(parent, allTxs)
}

// CreateSimpleBlock creates a block at height 1 with just coinbase (most common test case)
func CreateSimpleBlock() *blockchain.Block {
	return CreateTestBlockWithCoinbase(&blockchain.GenesisBlock.Header, config.FirstUser, []*blockchain.Transaction{})
}

// CreateBlockChainWithCoinbase creates a chain of blocks, each with coinbase
func CreateBlockChainWithCoinbase(length int, beneficiary blockchain.PublicKey) []*blockchain.Block {
	blocks := make([]*blockchain.Block, length)

	var parent *blockchain.BlockHeader = &blockchain.GenesisBlock.Header

	for i := 0; i < length; i++ {
		blocks[i] = CreateTestBlockWithCoinbase(parent, beneficiary, []*blockchain.Transaction{})
		parent = &blocks[i].Header
	}

	return blocks
}
