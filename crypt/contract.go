package crypt

import (
	"august/avm"
	"crypto/sha256"
)

// GenerateContractAddress generates a deterministic contract address from deployer and nonce
func GenerateContractAddress(deployer PublicKey, nonce NonceType) PublicKey {
	h := sha256.New()
	h.Write(deployer[:])
	h.Write(uint64ToBytes(nonce))
	hash := h.Sum(nil)

	var addr PublicKey
	copy(addr[:], hash[:32])
	return addr
}

// ComputeCodeHash calculates the hash of contract instructions
func ComputeCodeHash(instructions []avm.Instruction) Hash32 {
	if len(instructions) == 0 {
		return Hash32{}
	}

	h := sha256.New()
	for _, instruction := range instructions {
		h.Write([]byte{byte(instruction.Opcode)})
		if instruction.Value != nil {
			h.Write(instruction.Value.Bytes())
		}
		h.Write(uint32ToBytes(uint32(instruction.Param)))
	}

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

// ComputeStorageRoot calculates the Merkle root of persistent storage
func ComputeStorageRoot(persistent map[string]string) Hash32 {
	if len(persistent) == 0 {
		return Hash32{}
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

	var root Hash32
	copy(root[:], hashes[0])
	return root
}
