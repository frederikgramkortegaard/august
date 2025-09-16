package blockchain

import (
	"august/avm"
	"august/types"
	"august/utils"
	"crypto/sha256"
)

// GenerateContractAddress generates a deterministic contract address from deployer and nonce
func GenerateContractAddress(deployer PublicKey, nonce uint64) PublicKey {
	h := sha256.New()
	h.Write(deployer[:])
	h.Write(utils.Uint64ToBytes(nonce))
	hash := h.Sum(nil)

	var addr PublicKey
	copy(addr[:], hash[:32])
	return addr
}

// ComputeCodeHash calculates the hash of contract instructions
func ComputeCodeHash(instructions []avm.Instruction) types.Hash32 {
	if len(instructions) == 0 {
		return types.Hash32{}
	}

	h := sha256.New()
	for _, instruction := range instructions {
		h.Write([]byte{byte(instruction.Opcode)})
		if instruction.Value != nil {
			h.Write(instruction.Value.Bytes())
		}
		h.Write(utils.Uint32ToBytes(uint32(instruction.Param)))
	}

	var hash types.Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

// ComputeStorageRoot calculates the Merkle root of persistent storage
func ComputeStorageRoot(persistent map[string]string) types.Hash32 {
	if len(persistent) == 0 {
		return types.Hash32{}
	}

	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(persistent))
	for k := range persistent {
		keys = append(keys, k)
	}

	// Simple string sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	// Hash each key-value pair
	hashes := make([][]byte, len(keys))
	for i, key := range keys {
		h := sha256.New()
		h.Write([]byte(key))
		h.Write([]byte(persistent[key]))
		hashes[i] = h.Sum(nil)
	}

	// Build Merkle tree
	for len(hashes) > 1 {
		if len(hashes)%2 == 1 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}

		newLevel := make([][]byte, 0, len(hashes)/2)
		for i := 0; i < len(hashes); i += 2 {
			h := sha256.New()
			h.Write(hashes[i])
			h.Write(hashes[i+1])
			newLevel = append(newLevel, h.Sum(nil))
		}
		hashes = newLevel
	}

	var root types.Hash32
	copy(root[:], hashes[0])
	return root
}