package blockchain

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

const (
	Difficulty = 1
)

// FirstUser is the genesis account that receives the initial coin supply
// Generated from fixed seed in testing/generator.go (testing only!)
var FirstUser = PublicKey{
	0x99, 0x3f, 0xe6, 0xa3, 0x6d, 0x19, 0xed, 0x52,
	0x78, 0x90, 0xa7, 0x69, 0x8e, 0x94, 0x0c, 0x3c,
	0x1c, 0x62, 0x9c, 0xa0, 0x64, 0x43, 0x83, 0x78,
	0x71, 0xcb, 0x0e, 0xd1, 0x94, 0x35, 0x22, 0x9b,
}

type PublicKey [ed25519.PublicKeySize]byte // 32
type Signature [ed25519.SignatureSize]byte // 64

// Custom JSON marshaling for PublicKey
func (pk PublicKey) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(pk[:]) + `"`), nil
}

func (pk *PublicKey) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON string for PublicKey: %s", string(data))
	}

	b64String := string(data[1 : len(data)-1])
	decoded, err := base64.StdEncoding.DecodeString(b64String)
	if err != nil {
		return fmt.Errorf("failed to decode base64 '%s': %v", b64String, err)
	}

	if len(decoded) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid PublicKey size for '%s': got %d, want %d", b64String, len(decoded), ed25519.PublicKeySize)
	}

	copy(pk[:], decoded)
	return nil
}

// Custom JSON marshaling for Signature
func (sig Signature) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(sig[:]) + `"`), nil
}

func (sig *Signature) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON string for Signature")
	}

	decoded, err := base64.StdEncoding.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return fmt.Errorf("failed to decode base64: %v", err)
	}

	if len(decoded) != ed25519.SignatureSize {
		return fmt.Errorf("invalid Signature size: got %d, want %d", len(decoded), ed25519.SignatureSize)
	}

	copy(sig[:], decoded)
	return nil
}

type Transaction struct {
	From      PublicKey `json:"from"`
	To        PublicKey `json:"to"`
	Amount    uint64    `json:"amount"`
	Signature Signature `json:"signature"`
	Nonce     uint64    `json:"nonce"`
}

type Hash32 [32]byte

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

type BlockHeader struct {
	Version      uint64 `json:"version"`
	PreviousHash Hash32 `json:"previous_hash"`
	Height       uint64 `json:"height"`
	Timestamp    uint64 `json:"timestamp"`
	Nonce        uint64 `json:"nonce"`
	MerkleRoot   Hash32 `json:"merkle_root"`
}

type Block struct {
	Header       BlockHeader   `json:"header"`
	Transactions []Transaction `json:"transactions"`
}

type AccountState struct {
	Address PublicKey `json:"address"`
	Balance uint64    `json:"balance"`
	Nonce   uint64    `json:"nonce"`
}

type Chain struct {
	Blocks        []*Block                    `json:"blocks"`
	AccountStates map[PublicKey]*AccountState `json:"account_states"`
}
