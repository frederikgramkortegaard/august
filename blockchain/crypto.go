package blockchain

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
)

func uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// deterministic hash for transaction
func HashTransaction(tsx *Transaction) Hash32 {
	h := sha256.New()
	h.Write(tsx.From[:])
	h.Write(tsx.To[:])
	h.Write(uint64ToBytes(tsx.Fee))
	h.Write(uint64ToBytes(tsx.Amount))
	h.Write(uint64ToBytes(tsx.Nonce))
	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

func GetSigningBytesFromTransaction(tsx *Transaction) []byte {
	h := sha256.New()
	h.Write(tsx.From[:])
	h.Write(tsx.To[:])
	h.Write(uint64ToBytes(tsx.Fee))
	h.Write(uint64ToBytes(tsx.Amount))
	h.Write(uint64ToBytes(tsx.Nonce))
	return h.Sum(nil)
}

func SignTransaction(tsx *Transaction, privatekey []byte) []byte {
	signingbytes := GetSigningBytesFromTransaction(tsx)
	sig := ed25519.Sign(privatekey, signingbytes)
	copy(tsx.Signature[:], sig)
	return sig

}

// deterministic hash for block headers
func HashBlockHeader(header *BlockHeader) Hash32 {
	h := sha256.New()
	h.Write(uint64ToBytes(header.Version))
	h.Write(header.PreviousHash[:])
	h.Write(uint64ToBytes(header.Height))
	h.Write(uint64ToBytes(header.Timestamp))
	h.Write(header.MerkleRoot[:])
	h.Write([]byte(header.TotalWork)) // Write TotalWork as string bytes
	h.Write(uint64ToBytes(header.Nonce))
	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

// MerkleTransactions creates a merkle root from a list of transactions
func MerkleTransactions(transactions []Transaction) Hash32 {
	if len(transactions) == 0 {
		return Hash32{}
	}

	// Hash all transactions
	hashes := make([][]byte, len(transactions))
	for i, tx := range transactions {
		hash := HashTransaction(&tx)
		hashes[i] = hash[:]
	}

	// Build merkle tree
	for len(hashes) > 1 {
		// If odd number, duplicate last hash
		if len(hashes)%2 == 1 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}

		// Combine pairs
		newLevel := make([][]byte, 0, len(hashes)/2)
		for i := 0; i < len(hashes); i += 2 {
			h := sha256.New()
			h.Write(hashes[i])
			h.Write(hashes[i+1])
			newLevel = append(newLevel, h.Sum(nil))
		}
		hashes = newLevel
	}

	// Convert to Hash32
	var root Hash32
	copy(root[:], hashes[0])
	return root
}

// EncodeHash encodes a Hash32 to base64 string for network transmission
func EncodeHash(hash Hash32) string {
	return base64.StdEncoding.EncodeToString(hash[:])
}
