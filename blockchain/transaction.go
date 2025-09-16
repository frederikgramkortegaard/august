package blockchain

import (
	"august/avm"
	"august/utils"
	"crypto/ed25519"
	"crypto/sha256"
)

type Transaction struct {
	From             PublicKey         `json:"from"`
	To               PublicKey         `json:"to"`
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

// GetHash returns the hash of this transaction
func (t *Transaction) GetHash() Hash32 {
	h := sha256.New()
	h.Write(t.From[:])
	h.Write(t.To[:])
	h.Write(utils.Uint64ToBytes(t.Amount))
	h.Write(utils.Uint64ToBytes(t.Nonce))
	h.Write(utils.Uint64ToBytes(t.Timestamp))
	h.Write(utils.Uint64ToBytes(t.ChainID)) // Include ChainID for replay protection
	h.Write(utils.Uint64ToBytes(t.GasLimit)) // Include gas limit
	h.Write(utils.Uint64ToBytes(t.GasPrice)) // Include gas price

	// Hash the contract instructions if present
	if len(t.Instructions) > 0 {
		codeHash := ComputeCodeHash(t.Instructions)
		h.Write(codeHash[:])
	}

	// Hash the init instructions if present
	if len(t.InitInstructions) > 0 {
		initHash := ComputeCodeHash(t.InitInstructions)
		h.Write(initHash[:])
	}

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

// GetSignature computes the signature for this transaction using the given private key
func (t *Transaction) GetSignature(privateKey []byte) Signature {
	// Sign the transaction hash
	hash := t.GetHash()
	sig := ed25519.Sign(privateKey, hash[:])
	var signature Signature
	copy(signature[:], sig)
	return signature
}