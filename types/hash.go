package types

import (
	"encoding/base64"
	"fmt"
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
