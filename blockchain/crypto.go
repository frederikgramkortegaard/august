package blockchain

import (
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

// HashTransaction deterministically hashes a transaction
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

// GetSigningBytesFromTransaction returns bytes used for signing
func GetSigningBytesFromTransaction(tsx *Transaction) []byte {
	h := sha256.New()
	h.Write(tsx.From[:])
	h.Write(tsx.To[:])
	h.Write(uint64ToBytes(tsx.Fee))
	h.Write(uint64ToBytes(tsx.Amount))
	h.Write(uint64ToBytes(tsx.Nonce))
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

	// Encode TotalWork as big.Int bytes, not string
	workInt := new(big.Int)
	workInt.SetString(header.TotalWork, 10)
	h.Write(workInt.Bytes())

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
