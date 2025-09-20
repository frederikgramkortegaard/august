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

type Chain struct {
	mu            sync.RWMutex                `json:"-"` // RWMutex for thread safety
	Blocks        []*Block                    `json:"blocks"`
	AccountStates map[PublicKey]*AccountState `json:"account_states"`
	Tip           *Block                      `json:"-"` // Points to latest block for O(1) access
	BlockIndex    map[Hash32]*Block           `json:"-"` // Hash -> Block lookup for O(1) parent access
}

// DeepCopy creates a deep copy of the chain for safe concurrent validation
func (c *Chain) DeepCopy() *Chain {
	if c == nil {
		return nil
	}

	// Copy blocks slice
	blocks := make([]*Block, len(c.Blocks))
	for i, block := range c.Blocks {
		// Blocks are immutable once created, so shallow copy is safe
		blocks[i] = block
	}

	// Deep copy account states map
	accountStates := make(map[PublicKey]*AccountState)
	for pubKey, state := range c.AccountStates {
		if state != nil {
			// Deep copy Instructions slice
			var instructions []Instruction
			if state.Instructions != nil {
				instructions = make([]Instruction, len(state.Instructions))
				for i, instr := range state.Instructions {
					instructions[i] = Instruction{
						Opcode: instr.Opcode,
						Value:  nil,
					}
					// Deep copy the Value pointer if it exists
					if instr.Value != nil {
						valueCopy := *instr.Value  // Dereference and copy string
						instructions[i].Value = &valueCopy
					}
				}
			}

			// Deep copy Persistent storage map
			persistent := make(map[string]string)
			for k, v := range state.Persistent {
				persistent[k] = v
			}

			accountStates[pubKey] = &AccountState{
				Address:      state.Address,
				Balance:      state.Balance,
				Nonce:        state.Nonce,
				Instructions: instructions,
				Persistent:   persistent,
				StorageRoot:  state.StorageRoot,
				CodeHash:     state.CodeHash,
			}
		}
	}

	// Deep copy block index
	blockIndex := make(map[Hash32]*Block)
	for hash, block := range c.BlockIndex {
		blockIndex[hash] = block // Blocks are immutable, shallow copy is safe
	}

	// Set tip pointer
	var tip *Block
	if len(blocks) > 0 {
		tip = blocks[len(blocks)-1]
	}

	return &Chain{
		Blocks:        blocks,
		AccountStates: accountStates,
		Tip:           tip,
		BlockIndex:    blockIndex,
	}
}

// AddBlock adds a block to the chain and updates tip/index
func (c *Chain) AddBlock(block *Block) {
	c.Blocks = append(c.Blocks, block)
	c.Tip = block

	if c.BlockIndex == nil {
		c.BlockIndex = make(map[Hash32]*Block)
	}

	blockHash := block.GetHash()
	c.BlockIndex[blockHash] = block
}

// GetBlockByHash returns a block by its hash in O(1) time
func (c *Chain) GetBlockByHash(hash Hash32) (*Block, bool) {
	if c.BlockIndex == nil {
		return nil, false
	}
	block, exists := c.BlockIndex[hash]
	return block, exists
}

// InitializeIndexes builds the block index and sets tip for existing chains
func (c *Chain) InitializeIndexes() {
	c.BlockIndex = make(map[Hash32]*Block)

	for _, block := range c.Blocks {
		blockHash := block.GetHash()
		c.BlockIndex[blockHash] = block
	}

	if len(c.Blocks) > 0 {
		c.Tip = c.Blocks[len(c.Blocks)-1]
	}
}