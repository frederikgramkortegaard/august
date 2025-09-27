package networking

import (
	"august/blockchain"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// StringToHash32 converts a string (base64 or hex encoded) to a blockchain.Hash32
func StringToHash32(hashStr string) (blockchain.Hash32, error) {
	var hashBytes []byte
	var err error

	hashBytes, err = hex.DecodeString(hashStr)
	if err != nil {
		return blockchain.Hash32{}, fmt.Errorf("invalid hash format (tried base64 and hex): %w", err)
	}

	if len(hashBytes) != 32 {
		return blockchain.Hash32{}, fmt.Errorf("hash must be 32 bytes, got %d bytes", len(hashBytes))
	}

	var hash blockchain.Hash32
	copy(hash[:], hashBytes)
	return hash, nil
}

// Hash32ToString converts a blockchain.Hash32 to a base64 encoded string
func Hash32ToString(hash blockchain.Hash32) string {
	return base64.StdEncoding.EncodeToString(hash[:])
}
