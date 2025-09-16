package types

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
)

type Hash32 [32]byte
type Hash64 [64]byte

// Custom JSON marshaling for Hash32
func (h Hash32) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(h[:]) + `"`), nil
}

func (h *Hash32) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON string for Hash32")
	}

	decoded, err := base64.StdEncoding.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return fmt.Errorf("failed to decode base64: %v", err)
	}

	if len(decoded) != 32 {
		return fmt.Errorf("invalid Hash32 size: got %d, want 32", len(decoded))
	}

	copy(h[:], decoded)
	return nil
}

// GetHash returns the hash of the block header
func (bh *BlockHeader) GetHash() Hash32 {
	h := sha256.New()
	h.Write(uint64ToBytes(bh.Version))
	h.Write(bh.PreviousHash[:])
	h.Write(uint64ToBytes(bh.Height))
	h.Write(uint64ToBytes(bh.Timestamp))
	h.Write(bh.MerkleRoot[:])
	h.Write(bh.StateRoot[:])
	h.Write(bh.ReceiptRoot[:])
	h.Write(uint32ToBytes(bh.Bits))

	// Encode TotalWork as big.Int bytes, not string
	workInt := new(big.Int)
	workInt.SetString(bh.TotalWork, 10)
	h.Write(workInt.Bytes())

	h.Write(uint64ToBytes(bh.GasLimit))
	h.Write(uint64ToBytes(bh.GasUsed))
	h.Write(bh.Beneficiary[:])
	h.Write(uint64ToBytes(bh.Nonce))

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

// GetHash returns the hash of the transaction
func (tx *Transaction) GetHash() Hash32 {
	h := sha256.New()
	h.Write(tx.From[:])
	h.Write(tx.To[:])
	h.Write(uint64ToBytes(tx.Amount))
	h.Write(uint64ToBytes(tx.Nonce))
	h.Write(uint64ToBytes(tx.Timestamp))
	h.Write(uint64ToBytes(tx.ChainID))

	// Hash contract code if present
	if tx.Instructions != nil {
		for _, instr := range tx.Instructions {
			h.Write([]byte{byte(instr.Opcode)})
			if instr.Value != nil {
				valueBytes := instr.Value.Bytes()
				h.Write(uint16ToBytes(uint16(len(valueBytes))))
				h.Write(valueBytes)
			} else {
				h.Write(uint16ToBytes(0))
			}
			h.Write(uint16ToBytes(instr.Param))
		}
	}
	if tx.InitInstructions != nil {
		for _, instr := range tx.InitInstructions {
			h.Write([]byte{byte(instr.Opcode)})
			if instr.Value != nil {
				valueBytes := instr.Value.Bytes()
				h.Write(uint16ToBytes(uint16(len(valueBytes))))
				h.Write(valueBytes)
			} else {
				h.Write(uint16ToBytes(0))
			}
			h.Write(uint16ToBytes(instr.Param))
		}
	}

	h.Write(uint64ToBytes(tx.GasLimit))
	h.Write(uint64ToBytes(tx.GasPrice))

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

func uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

func uint32ToBytes(n uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return b
}

func uint16ToBytes(n uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, n)
	return b
}
