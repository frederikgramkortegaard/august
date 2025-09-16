package crypt

import (
	"august/blockchain"
	"crypto/sha256"
)

type Transaction blockchain.Transaction

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
