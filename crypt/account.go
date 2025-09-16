package crypt

import "crypto/sha256"

// HashAccountState computes the hash of an account state
func HashAccountState(state *AccountState) Hash32 {
	h := sha256.New()
	h.Write(state.Address[:])
	h.Write(uint64ToBytes(state.Balance))
	h.Write(uint64ToBytes(state.Nonce))

	// Hash the instructions (contract code)
	if len(state.Instructions) > 0 {
		// Serialize instructions and hash them
		for _, instruction := range state.Instructions {
			h.Write([]byte{byte(instruction.Opcode)})
			if instruction.Value != nil {
				h.Write(instruction.Value.Bytes())
			}
			h.Write(uint32ToBytes(uint32(instruction.Param)))
		}
	}

	// Hash the persistent storage
	h.Write(state.StorageRoot[:])
	h.Write(state.CodeHash[:])

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
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
		h := HashAccountState(state)
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
