package blockchain

import (
	"august/avm"
	"august/types"
)

type AccountState struct {
	Address      PublicKey         `json:"address"`
	Balance      uint64            `json:"balance"`
	Nonce        uint64            `json:"nonce"`
	Instructions []avm.Instruction `json:"instructions"`
	Persistent   map[string]string `json:"persistent"`
	StorageRoot  types.Hash32      `json:"storage_root"` // Merkle Patricia trie root of contract storage
	CodeHash     types.Hash32      `json:"code_hash"`    // Hash of the contract code (Instructions)
}