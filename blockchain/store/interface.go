package store

import (
	"gocuria/blockchain"
)

type ChainStore interface {

	// Update/Add/Put
	AddBlock(block *blockchain.Block) error
	ReplaceChain(newChain *blockchain.Chain) error

	// Getters
	GetBlockByHash(hash blockchain.Hash32) (*blockchain.Block, error)
	GetHeadBlock() (*blockchain.Block, error)
	GetAccountState(pubkey blockchain.PublicKey) (*blockchain.AccountState, error)
	GetChainHeight() (uint64, error)
	GetChain() (*blockchain.Chain, error)
	GetAccountStates() (map[blockchain.PublicKey]*blockchain.AccountState, error)

	// Locking for atomic operations
	Lock()
	Unlock()
	GetChainUnsafe() (*blockchain.Chain, error)          // Must be called with lock held
	ReplaceChainUnsafe(newChain *blockchain.Chain) error // Must be called with lock held
}
