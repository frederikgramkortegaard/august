package store

import (
	"errors"
	"fmt"
	"gocuria/blockchain"
	"sync"
)

type MemoryChainStore struct {
	chain *blockchain.Chain
	mu    sync.RWMutex
}

func NewMemoryChainStore() *MemoryChainStore {
	return &MemoryChainStore{
		chain: &blockchain.Chain{
			Blocks:        make([]*blockchain.Block, 0),
			AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
		},
	}
}

func (m *MemoryChainStore) AddBlock(block *blockchain.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain, err := m.getChainUnsafe()
	if err != nil {
		return fmt.Errorf("failed to get chain: %w", err)
	}
	if chain == nil {
		return errors.New("chain is nil")
	}

	// Just append the block to the chain - validation should be done by caller
	chain.Blocks = append(chain.Blocks, block)

	return nil
}

func (m *MemoryChainStore) GetHeadBlock() (*blockchain.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain, err := m.getChainUnsafe()
	if err != nil {
		return nil, err
	}

	if chain == nil {
		return nil, errors.New("chain is nil")
	}

	// Not returning an err as nil checks on this is valid
	if len(chain.Blocks) < 1 {
		return nil, nil
	}

	return chain.Blocks[len(chain.Blocks)-1], nil
}

func (m *MemoryChainStore) GetAccountState(pubkey blockchain.PublicKey) (*blockchain.AccountState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain, err := m.getChainUnsafe()
	if err != nil {
		return nil, err
	}

	if chain == nil {
		return nil, errors.New("chain is nil")
	}

	res, ok := chain.AccountStates[pubkey]
	if !ok {
		return nil, errors.New("account state for key not found")
	}

	return res, nil
}

func (m *MemoryChainStore) GetChain() (*blockchain.Chain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getChainUnsafe()
}

// getChainUnsafe returns the chain without locking - must be called with lock held
func (m *MemoryChainStore) getChainUnsafe() (*blockchain.Chain, error) {
	return m.chain, nil
}

func (m *MemoryChainStore) GetAccountStates() (map[blockchain.PublicKey]*blockchain.AccountState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain, err := m.getChainUnsafe()
	if err != nil {
		return nil, err
	}

	if chain == nil {
		return nil, errors.New("chain is nil")
	}

	return chain.AccountStates, nil
}

func (m *MemoryChainStore) GetBlockByHash(hash blockchain.Hash32) (*blockchain.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain, err := m.getChainUnsafe()
	if err != nil {
		return nil, err
	}

	if chain == nil {
		return nil, errors.New("chain is nil")
	}

	for _, block := range chain.Blocks {
		if blockchain.HashBlockHeader(&block.Header) == hash {
			return block, nil
		}
	}

	fmt.Printf("could not find block with hash %x\n", hash)
	return nil, nil
}

func (m *MemoryChainStore) GetChainHeight() (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain, err := m.getChainUnsafe()
	if err != nil {
		return 0, err
	}

	if chain == nil {
		return 0, errors.New("chain is nil")
	}

	return uint64(len(chain.Blocks)), nil
}

// ReplaceChain atomically replaces the entire chain - used after validation on copy
func (m *MemoryChainStore) ReplaceChain(newChain *blockchain.Chain) error {
	if newChain == nil {
		return errors.New("cannot replace with nil chain")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.chain = newChain
	return nil
}
