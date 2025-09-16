package blockchain

import (
	"august/utils"
	"crypto/sha256"
	"math/big"
)

type BlockHeader struct {
	Version      uint64    `json:"version"`
	PreviousHash Hash32    `json:"previous_hash"`
	Height       uint64    `json:"height"`
	Timestamp    uint64    `json:"timestamp"`
	Nonce        uint64    `json:"nonce"`
	MerkleRoot   Hash32    `json:"merkle_root"`     // Transaction merkle root
	StateRoot    Hash32    `json:"state_root"`      // Account state merkle root
	ReceiptRoot  Hash32    `json:"receipt_root"`    // Transaction receipts merkle root
	Bits         uint32    `json:"bits"`            // Target in compact format (Bitcoin-style)
	TotalWork    string    `json:"total_work"`      // Cumulative work from genesis to this block (big.Int as string)
	GasLimit     uint64    `json:"gas_limit"`       // Maximum gas allowed in this block
	GasUsed      uint64    `json:"gas_used"`        // Total gas used by all transactions
	Beneficiary  PublicKey `json:"beneficiary"`     // Miner/validator address receiving rewards
}

// GetHash returns the hash of this block header
func (h *BlockHeader) GetHash() Hash32 {
	hasher := sha256.New()
	hasher.Write(utils.Uint64ToBytes(h.Version))
	hasher.Write(h.PreviousHash[:])
	hasher.Write(utils.Uint64ToBytes(h.Height))
	hasher.Write(utils.Uint64ToBytes(h.Timestamp))
	hasher.Write(h.MerkleRoot[:])
	hasher.Write(h.StateRoot[:])
	hasher.Write(h.ReceiptRoot[:])
	hasher.Write(utils.Uint32ToBytes(h.Bits)) // Include Bits field!

	// Encode TotalWork as big.Int bytes, not string
	workInt := new(big.Int)
	workInt.SetString(h.TotalWork, 10)
	hasher.Write(workInt.Bytes())

	hasher.Write(utils.Uint64ToBytes(h.GasLimit))
	hasher.Write(utils.Uint64ToBytes(h.GasUsed))
	hasher.Write(h.Beneficiary[:])
	hasher.Write(utils.Uint64ToBytes(h.Nonce))

	var hash Hash32
	copy(hash[:], hasher.Sum(nil))
	return hash
}

type Block struct {
	Header       BlockHeader   `json:"header"`
	Transactions []Transaction `json:"transactions"`
}

// GetHash returns the hash of this block (which is the hash of its header)
func (b *Block) GetHash() Hash32 {
	return b.Header.GetHash()
}