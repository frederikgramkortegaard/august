package blockchain

import (
	"august/avm"
	"august/types"
	"august/utils"
	"crypto/sha256"
)

type Transaction struct {
	From             PublicKey         `json:"from"`
	To               PublicKey         `json:"to"`
	Amount           uint64            `json:"amount"`
	Signature        types.Signature   `json:"signature"`
	Nonce            uint64            `json:"nonce"`
	Timestamp        uint64            `json:"timestamp"`         // Unix timestamp when transaction was created
	ChainID          uint64            `json:"chain_id"`          // Chain identifier for replay protection
	Instructions     []avm.Instruction `json:"instructions"`      // Contract runtime code (empty for regular transfers)
	InitInstructions []avm.Instruction `json:"init_instructions"` // Constructor code that runs once at deployment
	GasLimit         uint64            `json:"gas_limit"`         // Maximum gas this transaction is willing to consume
	GasPrice         uint64            `json:"gas_price"`         // Price per unit of gas in leaf units
}

// GetHash returns the hash of the transaction
func (tx *Transaction) GetHash() types.Hash32 {
	h := sha256.New()
	h.Write(tx.From[:])
	h.Write(tx.To[:])
	h.Write(utils.Uint64ToBytes(tx.Amount))
	h.Write(utils.Uint64ToBytes(tx.Nonce))
	h.Write(utils.Uint64ToBytes(tx.Timestamp))
	h.Write(utils.Uint64ToBytes(tx.ChainID))

	// Hash contract code if present
	if tx.Instructions != nil {
		for _, instr := range tx.Instructions {
			h.Write([]byte{byte(instr.Opcode)})
			if instr.Value != nil {
				valueBytes := instr.Value.Bytes()
				h.Write(utils.Uint16ToBytes(uint16(len(valueBytes))))
				h.Write(valueBytes)
			} else {
				h.Write(utils.Uint16ToBytes(0))
			}
			h.Write(utils.Uint16ToBytes(instr.Param))
		}
	}
	if tx.InitInstructions != nil {
		for _, instr := range tx.InitInstructions {
			h.Write([]byte{byte(instr.Opcode)})
			if instr.Value != nil {
				valueBytes := instr.Value.Bytes()
				h.Write(utils.Uint16ToBytes(uint16(len(valueBytes))))
				h.Write(valueBytes)
			} else {
				h.Write(utils.Uint16ToBytes(0))
			}
			h.Write(utils.Uint16ToBytes(instr.Param))
		}
	}

	h.Write(utils.Uint64ToBytes(tx.GasLimit))
	h.Write(utils.Uint64ToBytes(tx.GasPrice))

	var hash types.Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}