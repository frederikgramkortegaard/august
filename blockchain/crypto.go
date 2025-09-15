package blockchain

import (
	"august/avm"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"math/big"
)

func uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

func uint32ToBytes(n uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return b
}

// HashTransaction deterministically hashes a transaction
func HashTransaction(tsx *Transaction) Hash32 {
	h := sha256.New()
	h.Write(tsx.From[:])
	h.Write(tsx.To[:])
	h.Write(uint64ToBytes(tsx.Amount))
	h.Write(uint64ToBytes(tsx.Nonce))
	h.Write(uint64ToBytes(tsx.Timestamp))
	h.Write(uint64ToBytes(tsx.ChainID)) // Include ChainID for replay protection
	h.Write(uint64ToBytes(tsx.GasLimit)) // Include gas limit
	h.Write(uint64ToBytes(tsx.GasPrice)) // Include gas price

	// Hash the contract instructions if present
	if len(tsx.Instructions) > 0 {
		codeHash := ComputeCodeHash(tsx.Instructions)
		h.Write(codeHash[:])
	}

	// Hash the init instructions if present
	if len(tsx.InitInstructions) > 0 {
		initHash := ComputeCodeHash(tsx.InitInstructions)
		h.Write(initHash[:])
	}

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

// GetSigningBytesFromTransaction returns bytes used for signing
func GetSigningBytesFromTransaction(tsx *Transaction) []byte {
	h := sha256.New()
	h.Write(tsx.From[:])
	h.Write(tsx.To[:])
	h.Write(uint64ToBytes(tsx.Amount))
	h.Write(uint64ToBytes(tsx.Nonce))
	h.Write(uint64ToBytes(tsx.Timestamp))
	h.Write(uint64ToBytes(tsx.GasLimit))
	h.Write(uint64ToBytes(tsx.GasPrice))
	return h.Sum(nil)
}

// SignTransaction signs the transaction with the given private key
func SignTransaction(tsx *Transaction, privateKey []byte) []byte {
	signingBytes := GetSigningBytesFromTransaction(tsx)
	sig := ed25519.Sign(privateKey, signingBytes)
	copy(tsx.Signature[:], sig)
	return sig
}

// HashBlockHeader deterministically hashes a block header
func HashBlockHeader(header *BlockHeader) Hash32 {
	h := sha256.New()
	h.Write(uint64ToBytes(header.Version))
	h.Write(header.PreviousHash[:])
	h.Write(uint64ToBytes(header.Height))
	h.Write(uint64ToBytes(header.Timestamp))
	h.Write(header.MerkleRoot[:])
	h.Write(header.StateRoot[:])
	h.Write(header.ReceiptRoot[:])
	h.Write(uint32ToBytes(header.Bits)) // Include Bits field!

	// Encode TotalWork as big.Int bytes, not string
	workInt := new(big.Int)
	workInt.SetString(header.TotalWork, 10)
	h.Write(workInt.Bytes())

	h.Write(uint64ToBytes(header.GasLimit))
	h.Write(uint64ToBytes(header.GasUsed))
	h.Write(header.Beneficiary[:])
	h.Write(uint64ToBytes(header.Nonce))

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

// MerkleTransactions computes a Merkle root for a list of transactions
func MerkleTransactions(transactions []Transaction) Hash32 {
	if len(transactions) == 0 {
		return Hash32{}
	}

	hashes := make([][]byte, len(transactions))
	for i, tx := range transactions {
		h := HashTransaction(&tx)
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
	h.Write(uint64ToBytes(nonce))
	hash := h.Sum(nil)

	var addr PublicKey
	copy(addr[:], hash[:32])
	return addr
}

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
