package crypt

import (
	"crypto/ed25519"
	"crypto/sha256"
)

func (tsx *Transaction) GetHash() Hash32 {
	h := sha256.New()
	h.Write(tsx.From[:])
	h.Write(tsx.To[:])
	h.Write(uint64ToBytes(tsx.Amount))
	h.Write(uint64ToBytes(tsx.Nonce))
	h.Write(uint64ToBytes(tsx.Timestamp))
	h.Write(uint64ToBytes(tsx.ChainID))
	h.Write(uint64ToBytes(tsx.GasLimit))
	h.Write(uint64ToBytes(tsx.GasPrice))

	// Hash the contract instructions if present
	if len(tsx.Instructions) > 0 {
		codeHash := ComputeCodeHash(tsx.Instructions)
		h.Write(codeHash[:])
	}

	// Hash the init instructions if present
	if len(tsx.InitInstructions) > 0 {
		initHash := ComputeCodeHash(tsx.InitInstructions)
		h.Write(initHash[:])
	}

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

// Returns the ed25519.Sign of Transaction.GetHash as a Hash64 ([64]byte)
func (tsx *Transaction) GetSignature(privateKey ed25519.PrivateKey) Hash64 {

	signingBytes := tsx.GetHash()
	sig := ed25519.Sign(privateKey, signingBytes)

	var signature Hash64
	copy(signature[:], sig)
	return sig
}

// Returns the result of ed25519.Verify of Transaction.GetHash and a Signature (Hash64)
func (tsx *Transaction) ValidateSignature(publicKey ed25519.PublicKey, signature Hash64) {
	hash := tsx.GetHash()
	return ed25519.Verify(publicKey, hash, signature)
}
