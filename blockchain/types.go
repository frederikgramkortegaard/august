package blockchain

import (
	"august/config"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

type PublicKey = config.PublicKey // Use the same type as config
type Signature [ed25519.SignatureSize]byte // 64



type Hash32 [32]byte

// String returns the hex string representation of the hash
func (h Hash32) String() string {
	return hex.EncodeToString(h[:])
}

// MarshalJSON implements json.Marshaler to encode as hex string
func (h Hash32) MarshalJSON() ([]byte, error) {
	return []byte(`"` + h.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler to decode from hex string
func (h *Hash32) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("hash must be a quoted hex string")
	}

	hexStr := string(data[1 : len(data)-1])
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return fmt.Errorf("invalid hex string: %w", err)
	}

	if len(bytes) != 32 {
		return fmt.Errorf("hash must be exactly 32 bytes, got %d", len(bytes))
	}

	copy(h[:], bytes)
	return nil
}

