package blockchain

import (
	"august/config"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

type PublicKey = config.PublicKey // Use the same type as config
type Signature [ed25519.SignatureSize]byte // 64

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