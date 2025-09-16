package types

// ChainHead represents the current head of the blockchain
type ChainHead struct {
	Height    uint64 `json:"height"`
	Hash      Hash32 `json:"hash"`
	TotalWork string `json:"total_work"`
}

// Our Blockchain Represenattion
type Chain struct {
	Blocks        []*Block                    `json:"blocks"`
	AccountStates map[PublicKey]*AccountState `json:"account_states"` // AccountStates not UTXO
	Tip           *Block                      `json:"-"`              // Points to latest block for O(1) access
	BlockIndex    map[Hash32]*Block           `json:"-"`              // Hash -> Block lookup for O(1) parent access
}
