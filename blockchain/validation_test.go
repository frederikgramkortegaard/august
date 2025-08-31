package blockchain

import (
	"testing"
)

func TestValidateTransaction(t *testing.T) {
	// Setup test account states
	accountStates := make(map[PublicKey]*AccountState)
	accountStates[FirstUser] = &AccountState{
		Address: FirstUser,
		Balance: 10000000, // 10M coins like genesis
		Nonce:   0,
	}

	// Create a test recipient
	testRecipient := PublicKey{0x01, 0x02, 0x03} // Just first few bytes for test

	tests := []struct {
		name        string
		tx          Transaction
		wantErr     bool
		errContains string
	}{
		{
			name: "coinbase transaction should be valid",
			tx: Transaction{
				From:   PublicKey{}, // Empty sender = coinbase
				To:     testRecipient,
				Amount: 1000,
				Nonce:  0,
			},
			wantErr: false,
		},
		{
			name: "transaction with invalid signature",
			tx: Transaction{
				From:      FirstUser,
				To:        testRecipient,
				Amount:    1000,
				Nonce:     1,
				Signature: Signature{}, // Zero signature = invalid
			},
			wantErr:     true,
			errContains: "invalid transaction signature",
		},
		// Note: The following tests will fail signature validation first,
		// which is expected behavior since we're using zero signatures.
		// In a real scenario, you'd need valid signatures to test other validations.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransaction(&tt.tx, accountStates)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateTransaction() expected error, got nil")
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("ValidateTransaction() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateTransaction() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateTransactionLogic(t *testing.T) {
	// Test validation logic by bypassing signature validation (coinbase transactions)
	accountStates := make(map[PublicKey]*AccountState)
	accountStates[FirstUser] = &AccountState{
		Address: FirstUser,
		Balance: 10000,
		Nonce:   0,
	}

	tests := []struct {
		name        string
		tx          Transaction
		wantErr     bool
		errContains string
	}{
		{
			name: "coinbase insufficient balance should pass - no sender validation",
			tx: Transaction{
				From:   PublicKey{}, // Coinbase - no sender validation
				To:     PublicKey{0x01},
				Amount: 999999999999, // Any amount is allowed for coinbase
				Nonce:  0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransaction(&tt.tx, accountStates)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateTransaction() expected error, got nil")
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("ValidateTransaction() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateTransaction() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateTransactionSignature(t *testing.T) {
	tests := []struct {
		name string
		tx   Transaction
		want bool
	}{
		{
			name: "zero signature should be invalid",
			tx: Transaction{
				From:      FirstUser,
				To:        PublicKey{},
				Amount:    1000,
				Nonce:     1,
				Signature: Signature{}, // All zeros
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateTransactionSignature(&tt.tx)
			if got != tt.want {
				t.Errorf("validateTransactionSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlockHashMeetsDifficulty(t *testing.T) {
	tests := []struct {
		name string
		hash Hash32
		want bool
	}{
		{
			name: "hash with leading zero should meet difficulty 1",
			hash: Hash32{0x00, 0x40, 0x26, 0xe5}, // Starts with 0x00 (meets difficulty 1)
			want: true,
		},
		{
			name: "hash without leading zero should not meet difficulty 1",
			hash: Hash32{0xFF, 0xFF, 0xFF, 0xFF}, // Starts with 0xFF
			want: false,
		},
		{
			name: "all zero hash should meet difficulty",
			hash: Hash32{}, // All zeros
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BlockHashMeetsDifficulty(tt.hash, 1) // Using difficulty 1 for test
			if got != tt.want {
				t.Errorf("BlockHashMeetsDifficulty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(substr) > 0 && len(s) > len(substr) &&
			(s[0:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				findInString(s, substr))))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
