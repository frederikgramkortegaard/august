package blockchain

import (
	"august/types"
)

// AddBlock adds a block to the chain and updates tip/index
func AddBlock(c *types.Chain, block *types.Block) {
	c.Blocks = append(c.Blocks, block)
	c.Tip = block

	if c.BlockIndex == nil {
		c.BlockIndex = make(map[types.Hash32]*types.Block)
	}

	blockHash := block.Header.GetHash()
	c.BlockIndex[blockHash] = block
}

// GetBlockByHash returns a block by its hash in O(1) time
func GetBlockByHash(c *types.Chain, hash types.Hash32) (*types.Block, bool) {
	if c.BlockIndex == nil {
		return nil, false
	}
	block, exists := c.BlockIndex[hash]
	return block, exists
}

// InitializeIndexes builds the block index and sets tip for existing chains
func InitializeIndexes(c *types.Chain) {
	c.BlockIndex = make(map[types.Hash32]*types.Block)

	for _, block := range c.Blocks {
		blockHash := block.Header.GetHash()
		c.BlockIndex[blockHash] = block
	}

	if len(c.Blocks) > 0 {
		c.Tip = c.Blocks[len(c.Blocks)-1]
	}
}
