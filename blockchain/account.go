package blockchain

import (
	"august/avm"
	"august/utils"
	"crypto/sha256"
)

type AccountState struct {
	Address           PublicKey         `json:"address"`
	Balance           uint64            `json:"balance"`
	Nonce             uint64            `json:"nonce"`
	Instructions      []avm.Instruction `json:"instructions"`
	Persistent        map[string]string `json:"persistent"`
	StorageRoot       Hash32            `json:"storage_root"`       // Merkle Patricia trie root of contract storage
	CodeHash          Hash32            `json:"code_hash"`          // Hash of the contract code (Instructions)
	DeploymentTsxHash Hash32            `json:"deployment_tsx_hash"` // Hash of transaction that deployed this contract
	DeploymentBlockHash Hash32          `json:"deployment_block_hash"` // Hash of block containing deployment transaction
}

// GetHash computes the hash of this account state
func (s *AccountState) GetHash() Hash32 {
	h := sha256.New()
	h.Write(s.Address[:])
	h.Write(utils.Uint64ToBytes(s.Balance))
	h.Write(utils.Uint64ToBytes(s.Nonce))

	// Hash the instructions (contract code)
	if len(s.Instructions) > 0 {
		// Serialize instructions and hash them
		for _, instruction := range s.Instructions {
			h.Write([]byte{byte(instruction.Opcode)})
			if instruction.Value != nil {
				h.Write([]byte(*instruction.Value))
			}
		}
	}

	// Hash the persistent storage
	h.Write(s.StorageRoot[:])
	h.Write(s.CodeHash[:])
	h.Write(s.DeploymentTsxHash[:])
	h.Write(s.DeploymentBlockHash[:])

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}