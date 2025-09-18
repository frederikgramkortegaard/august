package storage

import (
	"august/blockchain"
	"fmt"
	"sync"

	"github.com/cockroachdb/pebble"
)

// Store provides blockchain storage with in-memory chain and persistent Pebble backing
type Store struct {
	chain *blockchain.Chain
	db    *pebble.DB
	mu    sync.RWMutex
}

// NewStore creates a new storage instance that combines Pebble persistence with in-memory chain
func NewStore(dbname string) *Store {
	db, err := CreateOrLoadDatabase(dbname, &pebble.Options{})
	if err != nil {
		panic(fmt.Sprintf("Failed to open database '%s': %v", dbname, err))
	}

	// Load existing chain from disk if present
	chain, err := GetChain(db)
	if err != nil {
		// No existing chain, start with genesis
		chain = &blockchain.Chain{
			Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
			AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
		}
		// Apply genesis transactions to account states
		for _, tx := range blockchain.GenesisBlock.Transactions {
			blockchain.ValidateAndApplyTransaction(&tx, chain.AccountStates)
		}
	}

	return &Store{
		chain: chain,
		db:    db,
	}
}

// NewMemoryStore creates a new memory-only storage instance (no persistence)
func NewMemoryStore() *Store {
	chain := &blockchain.Chain{
		Blocks:        []*blockchain.Block{blockchain.GenesisBlock},
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	// Apply genesis transactions to account states
	for _, tx := range blockchain.GenesisBlock.Transactions {
		blockchain.ValidateAndApplyTransaction(&tx, chain.AccountStates)
	}

	return &Store{
		chain: chain,
		db:    nil, // No persistence
	}
}

// AddBlock adds a block to both memory and disk
func (s *Store) AddBlock(block *blockchain.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add to memory first
	if s.chain == nil {
		return fmt.Errorf("chain is nil")
	}

	// Append the block to the chain
	s.chain.Blocks = append(s.chain.Blocks, block)

	// Update block index
	if s.chain.BlockIndex == nil {
		s.chain.BlockIndex = make(map[blockchain.Hash32]*blockchain.Block)
	}
	blockHash := block.Header.GetHash()
	s.chain.BlockIndex[blockHash] = block

	// Update tip
	s.chain.Tip = block

	// Then persist to disk (if we have a database)
	if s.db != nil {
		if err := StoreBlock(s.db, block); err != nil {
			// Rollback memory change on disk failure
			s.chain.Blocks = s.chain.Blocks[:len(s.chain.Blocks)-1]
			delete(s.chain.BlockIndex, blockHash)
			if len(s.chain.Blocks) > 0 {
				s.chain.Tip = s.chain.Blocks[len(s.chain.Blocks)-1]
			} else {
				s.chain.Tip = nil
			}
			return fmt.Errorf("failed to persist block: %w", err)
		}
	}

	return nil
}

// ReplaceChain replaces the entire chain in both memory and disk
func (s *Store) ReplaceChain(newChain *blockchain.Chain) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.replaceChainUnsafe(newChain)
}

// ReplaceChainUnsafe replaces the chain without locking (caller must hold lock)
func (s *Store) ReplaceChainUnsafe(newChain *blockchain.Chain) error {
	return s.replaceChainUnsafe(newChain)
}

func (s *Store) replaceChainUnsafe(newChain *blockchain.Chain) error {
	if newChain == nil {
		return fmt.Errorf("cannot replace with nil chain")
	}

	// Replace in memory first
	s.chain = newChain

	// Store to disk (if we have a database)
	if s.db != nil {
		// Store any new blocks to disk (existing blocks already stored)
		for _, block := range newChain.Blocks {
			// This is idempotent - storing same block twice is fine
			if err := StoreBlock(s.db, block); err != nil {
				return fmt.Errorf("failed to persist block: %w", err)
			}
		}

		// Update the chain manifest (this is what actually "switches" chains)
		if err := StoreChainManifest(s.db, newChain); err != nil {
			return fmt.Errorf("failed to update chain manifest: %w", err)
		}
	}

	return nil
}

// GetBlockByHash retrieves a block by hash (from memory, fallback to disk)
func (s *Store) GetBlockByHash(hash blockchain.Hash32) (*blockchain.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try memory first (fast path)
	if s.chain != nil && s.chain.BlockIndex != nil {
		if block, ok := s.chain.BlockIndex[hash]; ok {
			return block, nil
		}
	}

	// Fallback to disk
	return GetBlock(s.db, hash)
}

// GetHeadBlock returns the current chain tip
func (s *Store) GetHeadBlock() (*blockchain.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.chain == nil || len(s.chain.Blocks) == 0 {
		return nil, nil
	}

	return s.chain.Blocks[len(s.chain.Blocks)-1], nil
}

// GetAccountState returns the state of a specific account
func (s *Store) GetAccountState(pubkey blockchain.PublicKey) (*blockchain.AccountState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.chain == nil {
		return nil, fmt.Errorf("chain is nil")
	}

	state, ok := s.chain.AccountStates[pubkey]
	if !ok {
		return nil, fmt.Errorf("account state for key not found")
	}

	return state, nil
}

// GetChainHeight returns the current chain height
func (s *Store) GetChainHeight() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.chain == nil {
		return 0, fmt.Errorf("chain is nil")
	}

	return uint64(len(s.chain.Blocks)), nil
}

// GetChain returns the complete chain (from memory), initializing with genesis if empty
func (s *Store) GetChain() (*blockchain.Chain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If chain is empty, initialize with genesis block
	if s.chain != nil && len(s.chain.Blocks) == 0 {
		// Add genesis block
		s.chain.Blocks = append(s.chain.Blocks, blockchain.GenesisBlock)

		// Update block index
		if s.chain.BlockIndex == nil {
			s.chain.BlockIndex = make(map[blockchain.Hash32]*blockchain.Block)
		}
		blockHash := blockchain.GenesisBlock.Header.GetHash()
		s.chain.BlockIndex[blockHash] = blockchain.GenesisBlock

		// Update tip
		s.chain.Tip = blockchain.GenesisBlock

		// Persist to disk if we have a database
		if s.db != nil {
			if err := StoreBlock(s.db, blockchain.GenesisBlock); err != nil {
				// Rollback on failure
				s.chain.Blocks = s.chain.Blocks[:0]
				delete(s.chain.BlockIndex, blockHash)
				s.chain.Tip = nil
				return nil, fmt.Errorf("failed to persist genesis block: %w", err)
			}
		}
	}

	return s.chain, nil
}

// GetChainUnsafe returns the chain without locking (caller must hold lock)
func (s *Store) GetChainUnsafe() (*blockchain.Chain, error) {
	return s.chain, nil
}

// GetAccountStates returns all account states
func (s *Store) GetAccountStates() (map[blockchain.PublicKey]*blockchain.AccountState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.chain == nil {
		return nil, fmt.Errorf("chain is nil")
	}

	return s.chain.AccountStates, nil
}

// GetChainHead returns information about the current chain head
func (s *Store) GetChainHead() blockchain.ChainHead {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If chain is empty, initialize with genesis block (same logic as GetChain)
	if s.chain != nil && len(s.chain.Blocks) == 0 {
		// Add genesis block
		s.chain.Blocks = append(s.chain.Blocks, blockchain.GenesisBlock)

		// Update block index
		if s.chain.BlockIndex == nil {
			s.chain.BlockIndex = make(map[blockchain.Hash32]*blockchain.Block)
		}
		blockHash := blockchain.GenesisBlock.Header.GetHash()
		s.chain.BlockIndex[blockHash] = blockchain.GenesisBlock

		// Update tip
		s.chain.Tip = blockchain.GenesisBlock

		// Persist to disk if we have a database
		if s.db != nil {
			if err := StoreBlock(s.db, blockchain.GenesisBlock); err != nil {
				// Rollback on failure
				s.chain.Blocks = s.chain.Blocks[:0]
				delete(s.chain.BlockIndex, blockHash)
				s.chain.Tip = nil
				// Return empty head on error
				return blockchain.ChainHead{
					Height:    0,
					Hash:      blockchain.Hash32{},
					TotalWork: "0",
				}
			}
		}
	}

	if s.chain == nil || len(s.chain.Blocks) == 0 {
		// Return empty chain info if still no blocks
		return blockchain.ChainHead{
			Height:    0,
			Hash:      blockchain.Hash32{},
			TotalWork: "0",
		}
	}

	lastBlock := s.chain.Blocks[len(s.chain.Blocks)-1]
	hash := lastBlock.Header.GetHash()

	return blockchain.ChainHead{
		Height:    lastBlock.Header.Height,
		Hash:      hash,
		TotalWork: lastBlock.Header.TotalWork,
	}
}

// Lock acquires the write lock
func (s *Store) Lock() {
	s.mu.Lock()
}

// Unlock releases the write lock
func (s *Store) Unlock() {
	s.mu.Unlock()
}

// Close closes the database connection (if any)
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}