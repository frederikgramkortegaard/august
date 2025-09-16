package crypt

import (
	"crypto/sha256"
	"math/big"
)

// HashBlockHeader deterministically hashes a block header
func (bh *BlockHeader) GetHash() Hash32 {
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
