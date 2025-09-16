package types

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

type NonceType = uint64

type PublicKey [ed25519.PublicKeySize]byte // 32 bytes, comparable for map keys
type PrivateKey ed25519.PrivateKey
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
