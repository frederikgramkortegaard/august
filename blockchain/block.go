package blockchain

import (
	"august/types"
	"august/utils"
	"crypto/sha256"
	"math/big"
)

type Block struct {
	Header       BlockHeader   `json:"header"`
	Transactions []Transaction `json:"transactions"`
}

type BlockHeader struct {
	Version      uint64            `json:"version"`
	PreviousHash types.Hash32      `json:"previous_hash"`
	Height       uint64            `json:"height"`
	Timestamp    uint64            `json:"timestamp"`
	Nonce        uint64            `json:"nonce"`
	MerkleRoot   types.Hash32      `json:"merkle_root"`  // Transaction merkle root
	StateRoot    types.Hash32      `json:"state_root"`   // Account state merkle root
	ReceiptRoot  types.Hash32      `json:"receipt_root"` // Transaction receipts merkle root
	Bits         uint32            `json:"bits"`         // Target in compact format (Bitcoin-style)
	TotalWork    string            `json:"total_work"`   // Cumulative work from genesis to this block (big.Int as string)
	GasLimit     uint64            `json:"gas_limit"`    // Maximum gas allowed in this block
	GasUsed      uint64            `json:"gas_used"`     // Total gas used by all transactions
	Beneficiary  PublicKey         `json:"beneficiary"`  // Miner/validator address receiving rewards
}

// GetHash returns the hash of the block header
func (bh *BlockHeader) GetHash() types.Hash32 {
	h := sha256.New()
	h.Write(utils.Uint64ToBytes(bh.Version))
	h.Write(bh.PreviousHash[:])
	h.Write(utils.Uint64ToBytes(bh.Height))
	h.Write(utils.Uint64ToBytes(bh.Timestamp))
	h.Write(bh.MerkleRoot[:])
	h.Write(bh.StateRoot[:])
	h.Write(bh.ReceiptRoot[:])
	h.Write(utils.Uint32ToBytes(bh.Bits))

	// Encode TotalWork as big.Int bytes, not string
	workInt := new(big.Int)
	workInt.SetString(bh.TotalWork, 10)
	h.Write(workInt.Bytes())

	h.Write(utils.Uint64ToBytes(bh.GasLimit))
	h.Write(utils.Uint64ToBytes(bh.GasUsed))
	h.Write(bh.Beneficiary[:])
	h.Write(utils.Uint64ToBytes(bh.Nonce))

	var hash types.Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}