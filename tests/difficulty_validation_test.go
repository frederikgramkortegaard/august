package tests

import (
	"august/blockchain"
	"fmt"
	"math/big"
	"testing"
)

func TestBlockHashMeetsDifficulty(t *testing.T) {
	t.Log("=== Testing BlockHashMeetsDifficulty with visual output ===")

	// Test cases with different hashes and difficulties
	testCases := []struct {
		name       string
		hash       blockchain.Hash32
		difficulty uint64
		expected   bool
	}{
		{
			name:       "Hash with 8 leading zeros vs difficulty 8",
			hash:       blockchain.Hash32{0x00, 0xFF}, // 00000000 11111111 ...
			difficulty: 8,
			expected:   true,
		},
		{
			name:       "Hash with 8 leading zeros vs difficulty 9",
			hash:       blockchain.Hash32{0x00, 0xFF}, // 00000000 11111111 ...
			difficulty: 9,
			expected:   false,
		},
		{
			name:       "Hash with 12 leading zeros vs difficulty 12",
			hash:       blockchain.Hash32{0x00, 0x0F}, // 00000000 00001111 ...
			difficulty: 12,
			expected:   true,
		},
		{
			name:       "Hash with 12 leading zeros vs difficulty 13",
			hash:       blockchain.Hash32{0x00, 0x0F}, // 00000000 00001111 ...
			difficulty: 13,
			expected:   false,
		},
		{
			name:       "Hash with 16 leading zeros vs difficulty 16",
			hash:       blockchain.Hash32{0x00, 0x00, 0xFF}, // 00000000 00000000 11111111 ...
			difficulty: 16,
			expected:   true,
		},
		{
			name:       "Hash with 16 leading zeros vs difficulty 17",
			hash:       blockchain.Hash32{0x00, 0x00, 0xFF}, // 00000000 00000000 11111111 ...
			difficulty: 17,
			expected:   false,
		},
		{
			name:       "Real-like hash with 20 leading zeros",
			hash:       blockchain.Hash32{0x00, 0x00, 0x0F, 0xAB, 0xCD}, // ~20 leading zeros
			difficulty: 20,
			expected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Print hash as hex
			t.Logf("Hash (hex): %x", tc.hash[:8]) // First 8 bytes for readability

			// Print hash as binary string (first 64 bits for readability)
			binaryStr := hashToBinaryString(tc.hash, 64)
			t.Logf("Hash (binary first 64 bits): %s", binaryStr)

			// Count actual leading zeros
			actualZeros := countLeadingZeros(tc.hash)
			t.Logf("Actual leading zero bits: %d", actualZeros)

			// Print difficulty requirement
			t.Logf("Required difficulty: %d", tc.difficulty)

			// Calculate and show target value
			target := new(big.Int).Lsh(big.NewInt(1), uint(256-tc.difficulty))
			t.Logf("Target value: 2^%d = %x", 256-tc.difficulty, target.Bytes()[:8])

			// Run the actual function
			// Convert difficulty to target bits for new system
		targetBits := blockchain.BigToCompact(new(big.Int).Lsh(big.NewInt(1), uint(256-tc.difficulty)))
		result := blockchain.BlockHashMeetsDifficulty(tc.hash, targetBits)
			t.Logf("Result: %v (expected: %v)", result, tc.expected)

			if result != tc.expected {
				t.Errorf("FAIL: Expected %v but got %v", tc.expected, result)
			} else {
				t.Logf("PASS: Correctly returned %v", result)
			}
			t.Log("---")
		})
	}
}

func TestMiningWithDifficulty(t *testing.T) {
	t.Log("=== Testing actual mining with visual output ===")

	// Create a test block header
	header := blockchain.BlockHeader{
		Version:      1,
		PreviousHash: blockchain.Hash32{},
		Height:       1,
		Timestamp:    1234567890,
		Nonce:        0,
		MerkleRoot:   blockchain.Hash32{0xAB, 0xCD},
		TotalWork:    "1",
	}

	// Test mining with low difficulty for speed
	difficulties := []uint64{8, 12, 16}

	for _, diff := range difficulties {
		t.Run(fmt.Sprintf("Mining with difficulty %d", diff), func(t *testing.T) {
			t.Logf("Starting mining with difficulty %d", diff)

			// Make a copy of header to not modify original
			headerCopy := header
			targetBits := blockchain.BigToCompact(new(big.Int).Lsh(big.NewInt(1), uint(256-diff)))
		nonce, err := blockchain.MineCorrectNonce(&headerCopy, targetBits)
			if err != nil {
				t.Fatalf("Mining failed: %v", err)
			}

			// Calculate final hash
			headerCopy.Nonce = nonce
			hash := blockchain.HashBlockHeader(&headerCopy)

			// Display results
			t.Logf("Found nonce: %d", nonce)
			t.Logf("Final hash (hex): %x", hash[:8])
			
			binaryStr := hashToBinaryString(hash, 32)
			t.Logf("Final hash (binary first 32 bits): %s", binaryStr)
			
			actualZeros := countLeadingZeros(hash)
			t.Logf("Actual leading zero bits: %d (required: %d)", actualZeros, diff)

			// Verify it meets difficulty
			if !blockchain.BlockHashMeetsDifficulty(hash, targetBits) {
				t.Errorf("Mined hash doesn't meet difficulty!")
			} else {
				t.Logf("PASS: Hash meets difficulty requirement")
			}
		})
	}
}

// Helper function to convert hash to binary string
func hashToBinaryString(hash blockchain.Hash32, bits int) string {
	result := ""
	bytesToShow := bits / 8
	if bytesToShow > 32 {
		bytesToShow = 32
	}
	
	for i := 0; i < bytesToShow; i++ {
		if i > 0 {
			result += " "
		}
		result += fmt.Sprintf("%08b", hash[i])
	}
	return result
}

// Helper function to count leading zeros
func countLeadingZeros(hash blockchain.Hash32) int {
	zeros := 0
	for _, b := range hash {
		if b == 0 {
			zeros += 8
		} else {
			// Count leading zeros in this byte
			for i := 7; i >= 0; i-- {
				if (b & (1 << i)) == 0 {
					zeros++
				} else {
					return zeros
				}
			}
		}
	}
	return zeros
}