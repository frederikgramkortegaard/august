package blockchain

import (
	"august/types"
)

// ChainHead represents the current head of the blockchain
type ChainHead struct {
	Height    uint64       `json:"height"`
	Hash      types.Hash32 `json:"hash"`
	TotalWork string       `json:"total_work"`
}

// Our Blockchain Represenattion
type Chain struct {
	Blocks        []*Block                    `json:"blocks"`
	AccountStates map[PublicKey]*AccountState `json:"account_states"` // AccountStates not UTXO
	Tip           *Block                      `json:"-"`              // Points to latest block for O(1) access
	BlockIndex    map[types.Hash32]*Block     `json:"-"`              // Hash -> Block lookup for O(1) parent access
}

// AddBlock adds a block to the chain and updates tip/index
func (c *Chain) AddBlock(block *Block) {
	c.Blocks = append(c.Blocks, block)
	c.Tip = block

	if c.BlockIndex == nil {
		c.BlockIndex = make(map[types.Hash32]*Block)
	}

	blockHash := block.Header.GetHash()
	c.BlockIndex[blockHash] = block
}

// GetBlockByHash returns a block by its hash in O(1) time
func (c *Chain) GetBlockByHash(hash types.Hash32) (*Block, bool) {
	if c.BlockIndex == nil {
		return nil, false
	}
	block, exists := c.BlockIndex[hash]
	return block, exists
}

// InitializeIndexes builds the block index and sets tip for existing chains
func (c *Chain) InitializeIndexes() {
	c.BlockIndex = make(map[types.Hash32]*Block)

	for _, block := range c.Blocks {
		blockHash := block.Header.GetHash()
		c.BlockIndex[blockHash] = block
	}

	if len(c.Blocks) > 0 {
		c.Tip = c.Blocks[len(c.Blocks)-1]
	}
}