package types

type Block struct {
	Header       BlockHeader   `json:"header"`
	Transactions []Transaction `json:"transactions"`
}
type BlockHeader struct {
	Version      uint64    `json:"version"`
	PreviousHash Hash32    `json:"previous_hash"`
	Height       uint64    `json:"height"`
	Timestamp    uint64    `json:"timestamp"`
	Nonce        uint64    `json:"nonce"`
	MerkleRoot   Hash32    `json:"merkle_root"`  // Transaction merkle root
	StateRoot    Hash32    `json:"state_root"`   // Account state merkle root
	ReceiptRoot  Hash32    `json:"receipt_root"` // Transaction receipts merkle root
	Bits         uint32    `json:"bits"`         // Target in compact format (Bitcoin-style)
	TotalWork    string    `json:"total_work"`   // Cumulative work from genesis to this block (big.Int as string)
	GasLimit     uint64    `json:"gas_limit"`    // Maximum gas allowed in this block
	GasUsed      uint64    `json:"gas_used"`     // Total gas used by all transactions
	Beneficiary  PublicKey `json:"beneficiary"`  // Miner/validator address receiving rewards
}
