package blockchain

// ErrMissingParent is returned when a block's parent is not found in the chain
type ErrMissingParent struct {
	Hash Hash32
}

func (e ErrMissingParent) Error() string { return "missing parent" }

// ErrSwitchChain is returned when a block triggers a chain reorganization
type ErrSwitchChain struct {
	Block          *Block
	CommonAncestor *Block
}

func (e ErrSwitchChain) Error() string { return "chain switch" }