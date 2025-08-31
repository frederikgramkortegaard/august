package store

import (
	"gocuria/blockchain"
)

type ChainStore interface {

	// Update/Add/Put
	AddBlock(block *blockchain.Block) error

	// Getters
	GetBlockByHash(hash blockchain.Hash32) (*blockchain.Block, error)
	GetHeadBlock() (*blockchain.Block, error)
	GetAccountState(pubkey blockchain.PublicKey) (*blockchain.AccountState, error)
	GetChainHeight() (uint64, error)
	GetChain() (*blockchain.Chain, error)
	GetAccountStates() (map[blockchain.PublicKey]*blockchain.AccountState, error)
}