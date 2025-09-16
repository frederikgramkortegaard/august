package types

import (
	"august/avm"
	"crypto/ed25519"
)

type Transaction struct {
	From             ed25519.PublicKey `json:"from"`
	To               ed25519.PublicKey `json:"to"`
	Amount           uint64            `json:"amount"`
	Signature        Signature         `json:"signature"`
	Nonce            uint64            `json:"nonce"`
	Timestamp        uint64            `json:"timestamp"`         // Unix timestamp when transaction was created
	ChainID          uint64            `json:"chain_id"`          // Chain identifier for replay protection
	Instructions     []avm.Instruction `json:"instructions"`      // Contract runtime code (empty for regular transfers)
	InitInstructions []avm.Instruction `json:"init_instructions"` // Constructor code that runs once at deployment
	GasLimit         uint64            `json:"gas_limit"`         // Maximum gas this transaction is willing to consume
	GasPrice         uint64            `json:"gas_price"`         // Price per unit of gas in leaf units
}
