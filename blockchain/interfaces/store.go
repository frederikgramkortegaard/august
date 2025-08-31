package interfaces

// ChainStore defines the interface for blockchain storage
// We use interface{} to avoid import cycles - implementations should type assert
type ChainStore interface {
	// Update/Add/Put
	AddBlock(block interface{}) error

	// Getters
	GetBlockByHash(hash interface{}) (interface{}, error)
	GetHeadBlock() (interface{}, error)
	GetAccountState(pubkey interface{}) (interface{}, error)
	GetChainHeight() (uint64, error)
	GetChain() (interface{}, error)
	GetAccountStates() (interface{}, error)
}