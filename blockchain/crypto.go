package blockchain

import (
	"august/utils"
	"crypto/sha256"
	"encoding/base64"
)

// MerkleTransactions computes a Merkle root for a list of transactions
func MerkleTransactions(transactions []Transaction) Hash32 {
	if len(transactions) == 0 {
		return Hash32{}
	}

	hashes := make([][]byte, len(transactions))
	for i, tx := range transactions {
		h := tx.GetHash()
		hashes[i] = h[:]
	}

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

// EncodeHash converts a Hash32 to base64 string
func EncodeHash(hash Hash32) string {
	return base64.StdEncoding.EncodeToString(hash[:])
}

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

// ComputeStateRoot calculates the Merkle root of all account states
func ComputeStateRoot(accountStates map[PublicKey]*AccountState) Hash32 {
	if len(accountStates) == 0 {
		return Hash32{}
	}

	// Sort accounts by address for deterministic ordering
	addresses := make([]PublicKey, 0, len(accountStates))
	for addr := range accountStates {
		addresses = append(addresses, addr)
	}

	// Sort addresses lexicographically
	for i := 0; i < len(addresses); i++ {
		for j := i + 1; j < len(addresses); j++ {
			for k := 0; k < 32; k++ {
				if addresses[i][k] > addresses[j][k] {
					addresses[i], addresses[j] = addresses[j], addresses[i]
					break
				} else if addresses[i][k] < addresses[j][k] {
					break
				}
			}
		}
	}

	// Create hashes for each account state
	hashes := make([][]byte, len(addresses))
	for i, addr := range addresses {
		state := accountStates[addr]
		h := state.GetHash()
		hashes[i] = h[:]
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

// ComputeCodeHash calculates the hash of contract instructions
func ComputeCodeHash(instructions []Instruction) Hash32 {
	if len(instructions) == 0 {
		return Hash32{}
	}

	h := sha256.New()
	for _, instruction := range instructions {
		h.Write([]byte{byte(instruction.Opcode)})
		if instruction.Value != nil {
			h.Write([]byte(*instruction.Value))
		}
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
