package blockchain

import (
	"sync"
)

// ChainHead represents the current head of the blockchain
type ChainHead struct {
	Height    uint64 `json:"height"`
	Hash      Hash32 `json:"hash"`
	TotalWork string `json:"total_work"`
}

// Chain represents a complete blockchain with its own state
// Each chain manages its blocks, headers, and account states independently
type Chain struct {
	// Chain data (public for backward compatibility with validation.go)
	Blocks        []*Block                    `json:"blocks"`         // Height-indexed blocks
	BlockIndex    map[Hash32]*Block           `json:"-"`              // Hash -> Block lookup
	Headers       map[Hash32]*BlockHeader     `json:"-"`              // Hash -> Header lookup
	AccountStates map[PublicKey]*AccountState `json:"account_states"` // Account states
	Tip           *Block                      `json:"-"`              // Current tip

	// Mutexes for thread safety
	blocksMu        sync.RWMutex
	headersMu       sync.RWMutex
	accountStatesMu sync.RWMutex
	heightIndexMu   sync.RWMutex
}

// GetBlock returns a block by hash
func (c *Chain) GetBlock(hash Hash32) *Block {
	c.blocksMu.RLock()
	defer c.blocksMu.RUnlock()
	return c.BlockIndex[hash]
}

// GetHeader returns a header by hash
func (c *Chain) GetHeader(hash Hash32) *BlockHeader {
	c.headersMu.RLock()
	defer c.headersMu.RUnlock()
	return c.Headers[hash]
}

// GetBlockAtHeight returns a block at a specific height
func (c *Chain) GetBlockAtHeight(height uint64) *Block {
	c.heightIndexMu.RLock()
	defer c.heightIndexMu.RUnlock()
	if height >= uint64(len(c.Blocks)) {
		return nil
	}
	return c.Blocks[height]
}

// AddHeader adds a header to the chain
func (c *Chain) AddHeader(header *BlockHeader) {
	hash := header.GetHash()
	c.headersMu.Lock()
	c.Headers[hash] = header
	c.headersMu.Unlock()
}

// AddBlock adds a block to the chain and updates indices
func (c *Chain) AddBlock(block *Block) {
	hash := block.Header.GetHash()

	// Add to block index
	c.blocksMu.Lock()
	c.BlockIndex[hash] = block
	c.blocksMu.Unlock()

	// Add header
	c.AddHeader(&block.Header)

	// Update height index
	c.heightIndexMu.Lock()
	// Ensure blocks slice is large enough
	for uint64(len(c.Blocks)) <= block.Header.Height {
		c.Blocks = append(c.Blocks, nil)
	}
	c.Blocks[block.Header.Height] = block
	c.heightIndexMu.Unlock()

	// Update tip if this block is at the end of the chain
	if c.Tip == nil || block.Header.Height > c.Tip.Header.Height {
		c.Tip = block
	}
}

// GetTip returns the current tip of the chain
func (c *Chain) GetTip() *Block {
	return c.Tip
}

// GetTipHash returns the hash of the current tip
func (c *Chain) GetTipHash() Hash32 {
	if c.Tip == nil {
		return Hash32{}
	}
	return c.Tip.Header.GetHash()
}

// GetAccountStates returns a copy of the account states
func (c *Chain) GetAccountStates() map[PublicKey]*AccountState {
	c.accountStatesMu.RLock()
	defer c.accountStatesMu.RUnlock()

	states := make(map[PublicKey]*AccountState)
	for k, v := range c.AccountStates {
		states[k] = v
	}
	return states
}

// SetAccountStates updates the account states
func (c *Chain) SetAccountStates(states map[PublicKey]*AccountState) {
	c.accountStatesMu.Lock()
	c.AccountStates = states
	c.accountStatesMu.Unlock()
}

// GetBlockByHash returns a block by its hash (backward compatibility)
func (c *Chain) GetBlockByHash(hash Hash32) (*Block, bool) {
	block := c.GetBlock(hash)
	return block, block != nil
}

// NewChain creates a new chain
func NewChain() *Chain {
	chain := &Chain{
		Blocks:        make([]*Block, 0),
		BlockIndex:    make(map[Hash32]*Block),
		Headers:       make(map[Hash32]*BlockHeader),
		AccountStates: make(map[PublicKey]*AccountState),
	}

	// Always include genesis block in new chains
	genesisHash := GenesisBlock.Header.GetHash()
	chain.BlockIndex[genesisHash] = GenesisBlock
	chain.Headers[genesisHash] = &GenesisBlock.Header

	return chain
}
