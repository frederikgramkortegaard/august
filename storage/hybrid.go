package storage

import (
	"august/blockchain"
	"fmt"
	"sync"

	"github.com/cockroachdb/pebble"
)

type PersistentChainStore struct {
	mem *MemoryChainStore
	db  *pebble.DB
	mu  sync.RWMutex
}

// Hybrid chain store, keeps both in memory and persistent on-disk
func NewPersistentChainStore(dbname string) *PersistentChainStore {
	db, err := CreateOrLoadDatabase(dbname, &pebble.Options{})
	if err != nil {
		panic(fmt.Sprintf("Failed to open database '%s': %v", dbname, err))
	}

	// Load existing chain from disk if present
	chain, err := GetChain(db)
	if err != nil {
		// No existing chain, start with empty
		chain = &blockchain.Chain{
			Blocks:        make([]*blockchain.Block, 0),
			AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
		}
	}

	// Initialize memory store with loaded chain
	mem := NewMemoryChainStore()
	mem.chain = chain

	return &PersistentChainStore{
		mem: mem,
		db:  db,
	}
}

// AddBlock adds a block to both memory and disk
func (s *PersistentChainStore) AddBlock(block *blockchain.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add to memory first
	if err := s.mem.AddBlock(block); err != nil {
		return err
	}

	// Then persist to disk
	if err := StoreBlock(s.db, block); err != nil {
		// Rollback memory change on disk failure
		// This is simplified - in production you'd want proper rollback
		return fmt.Errorf("failed to persist block: %w", err)
	}

	return nil
}

// ReplaceChain replaces the entire chain in both memory and disk
func (s *PersistentChainStore) ReplaceChain(newChain *blockchain.Chain) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Replace in memory first
	if err := s.mem.ReplaceChain(newChain); err != nil {
		return err
	}

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

	return nil
}

// GetBlockByHash retrieves a block by hash (from memory, fallback to disk)
func (s *PersistentChainStore) GetBlockByHash(hash blockchain.Hash32) (*blockchain.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try memory first (fast path)
	block, err := s.mem.GetBlockByHash(hash)
	if err == nil {
		return block, nil
	}

	// Fallback to disk
	return GetBlock(s.db, hash)
}

// GetHeadBlock returns the current chain tip
func (s *PersistentChainStore) GetHeadBlock() (*blockchain.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.mem.GetHeadBlock()
}

// GetAccountState returns the state of a specific account
func (s *PersistentChainStore) GetAccountState(pubkey blockchain.PublicKey) (*blockchain.AccountState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.mem.GetAccountState(pubkey)
}

// GetChainHeight returns the current chain height
func (s *PersistentChainStore) GetChainHeight() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.mem.GetChainHeight()
}

// GetChain returns the complete chain (from memory)
func (s *PersistentChainStore) GetChain() (*blockchain.Chain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.mem.GetChain()
}

// GetAccountStates returns all account states
func (s *PersistentChainStore) GetAccountStates() (map[blockchain.PublicKey]*blockchain.AccountState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.mem.GetAccountStates()
}

// Lock acquires the write lock
func (s *PersistentChainStore) Lock() {
	s.mu.Lock()
}

// Unlock releases the write lock
func (s *PersistentChainStore) Unlock() {
	s.mu.Unlock()
}

// GetChainUnsafe returns the chain without locking (caller must hold lock)
func (s *PersistentChainStore) GetChainUnsafe() (*blockchain.Chain, error) {
	return s.mem.GetChainUnsafe()
}

// ReplaceChainUnsafe replaces the chain without locking (caller must hold lock)
func (s *PersistentChainStore) ReplaceChainUnsafe(newChain *blockchain.Chain) error {
	// Replace in memory
	if err := s.mem.ReplaceChainUnsafe(newChain); err != nil {
		return err
	}

	// Store any new blocks (idempotent)
	for _, block := range newChain.Blocks {
		if err := StoreBlock(s.db, block); err != nil {
			return fmt.Errorf("failed to persist block: %w", err)
		}
	}

	// Update chain manifest
	if err := StoreChainManifest(s.db, newChain); err != nil {
		return fmt.Errorf("failed to update chain manifest: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *PersistentChainStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}