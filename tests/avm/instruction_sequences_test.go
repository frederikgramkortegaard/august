package avm_test

import (
	"august/avm"
	"math/big"
	"testing"
)

// Helper function for creating hex values
func hex(s string) *big.Int {
	n, _ := new(big.Int).SetString(s, 16)
	return n
}

func TestSwapOperation(t *testing.T) {
	// Test SWAP operation: [A, B, C, D] -> SWAP 2 -> [A, D, C, B]
	instructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // Stack: [A]
		avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // Stack: [A, B]
		avm.MakeInstructionWithValue(avm.PUSH, hex("C")), // Stack: [A, B, C]
		avm.MakeInstructionWithValue(avm.PUSH, hex("D")), // Stack: [A, B, C, D]
		avm.MakeInstructionWithParam(avm.SWAP, 2),        // Stack: [A, D, C, B] (swap top with 2nd from top)
	}

	r := avm.NewRuntime(1000)
	gasUsed, err := r.StartExecution(instructions)

	if err != nil {
		t.Fatalf("Execution error: %v", err)
	}

	// Verify gas consumption
	expectedGas := uint64(3*4 + 3) // 4 PUSH operations (3 gas each) + 1 SWAP (3 gas)
	if gasUsed != expectedGas {
		t.Errorf("Expected gas %d, got %d", expectedGas, gasUsed)
	}

	// Verify stack state by popping elements in order
	// Stack should be [A, D, C, B] from bottom to top
	expectedValues := []*big.Int{hex("B"), hex("C"), hex("D"), hex("A")}

	for i, expected := range expectedValues {
		val, err := r.Stack.Pop()
		if err != nil {
			t.Fatalf("Failed to pop element %d: %v", i, err)
		}
		if val.Cmp(expected) != 0 {
			t.Errorf("Element %d: expected %x, got %x", i, expected, val)
		}
	}
}

func TestBasicStackOperations(t *testing.T) {
	// Test basic push, pop, dup operations
	instructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, hex("42")), // Stack: [42]
		avm.MakeInstruction(avm.DUP),                      // Stack: [42, 42]
		avm.MakeInstructionWithValue(avm.PUSH, hex("100")), // Stack: [42, 42, 100]
		avm.MakeInstruction(avm.POP),                      // Stack: [42, 42]
	}

	r := avm.NewRuntime(100)
	gasUsed, err := r.StartExecution(instructions)

	if err != nil {
		t.Fatalf("Execution error: %v", err)
	}

	// Verify final stack has two 42s
	val1, err := r.Stack.Pop()
	if err != nil || val1.Cmp(hex("42")) != 0 {
		t.Errorf("Expected top element to be 42, got %v (err: %v)", val1, err)
	}

	val2, err := r.Stack.Pop()
	if err != nil || val2.Cmp(hex("42")) != 0 {
		t.Errorf("Expected second element to be 42, got %v (err: %v)", val2, err)
	}

	// Stack should be empty now
	_, err = r.Stack.Pop()
	if err == nil {
		t.Error("Expected stack to be empty")
	}

	// Verify gas consumption: 2 PUSH (6) + 1 DUP (3) + 1 POP (2) = 11
	expectedGas := uint64(11)
	if gasUsed != expectedGas {
		t.Errorf("Expected gas %d, got %d", expectedGas, gasUsed)
	}
}

func TestSwapEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setup       []avm.Instruction
		swapDown    string
		shouldError bool
	}{
		{
			name: "swap with insufficient stack",
			setup: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex("1")),
				avm.MakeInstructionWithValue(avm.PUSH, hex("2")),
				avm.MakeInstructionWithParam(avm.SWAP, 3), // trying to swap 3 down with only 2 elements
			},
			shouldError: true,
		},
		{
			name: "swap 0 (should be valid)",
			setup: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex("1")),
				avm.MakeInstructionWithValue(avm.PUSH, hex("2")),
				avm.MakeInstructionWithParam(avm.SWAP, 0), // swap with self (no-op)
			},
			shouldError: false,
		},
		{
			name: "swap 1 (swap top two)",
			setup: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex("1")),
				avm.MakeInstructionWithValue(avm.PUSH, hex("2")),
				avm.MakeInstructionWithParam(avm.SWAP, 1), // swap top two elements
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := avm.NewRuntime(1000)

			// Execute all instructions including the swap
			_, err := r.StartExecution(tt.setup)

			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestGasAccounting(t *testing.T) {
	// Test that gas is properly consumed and tracked
	instructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, hex("1")), // 3 gas
		avm.MakeInstructionWithValue(avm.PUSH, hex("2")), // 3 gas
		avm.MakeInstruction(avm.DUP),                     // 3 gas
		avm.MakeInstructionWithParam(avm.SWAP, 1),        // 3 gas
		avm.MakeInstruction(avm.POP),                     // 2 gas
		// Total: 14 gas
	}

	r := avm.NewRuntime(20) // Enough gas
	gasUsed, err := r.StartExecution(instructions)

	if err != nil {
		t.Fatalf("Execution error: %v", err)
	}

	expectedGas := uint64(14)
	if gasUsed != expectedGas {
		t.Errorf("Expected gas %d, got %d", expectedGas, gasUsed)
	}

	if r.GasAvailable != 6 { // 20 - 14 = 6
		t.Errorf("Expected remaining gas 6, got %d", r.GasAvailable)
	}
}

func TestOutOfGas(t *testing.T) {
	// Test running out of gas
	instructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, hex("1")), // 3 gas
		avm.MakeInstructionWithValue(avm.PUSH, hex("2")), // 3 gas
		avm.MakeInstructionWithValue(avm.PUSH, hex("3")), // 3 gas - this should fail
	}

	r := avm.NewRuntime(5) // Not enough gas (need 9)
	_, err := r.StartExecution(instructions)

	if err == nil {
		t.Error("Expected out of gas error")
	}

	// Should be able to identify it as an out of gas error
	if err != avm.ErrOutOfGas {
		t.Errorf("Expected ErrOutOfGas, got %v", err)
	}
}

func TestInstructionValidation(t *testing.T) {
	tests := []struct {
		name        string
		instruction avm.Instruction
		shouldError bool
		expectedErr error
	}{
		{
			name:        "valid PUSH",
			instruction: avm.MakeInstructionWithValue(avm.PUSH, hex("42")),
			shouldError: false,
		},
		{
			name:        "PUSH with negative value",
			instruction: avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(-1)),
			shouldError: true,
			expectedErr: avm.ErrNegativeValue,
		},
		{
			name:        "PUSH with nil value",
			instruction: avm.MakeInstructionWithValue(avm.PUSH, nil),
			shouldError: true,
			expectedErr: avm.ErrInvalidPush,
		},
		{
			name:        "PUSH with unexpected param",
			instruction: avm.MakeInstructionWithValueAndParam(avm.PUSH, hex("42"), 1),
			shouldError: true,
			expectedErr: avm.ErrUnexpectedInstructionParameter,
		},
		{
			name:        "valid SWAP",
			instruction: avm.MakeInstructionWithParam(avm.SWAP, 2),
			shouldError: false,
		},
		{
			name:        "SWAP with too large param",
			instruction: avm.MakeInstructionWithParam(avm.SWAP, 2000),
			shouldError: true,
			expectedErr: avm.ErrInvalidSwapParameter,
		},
		{
			name:        "SWAP with unexpected value",
			instruction: avm.MakeInstructionWithValueAndParam(avm.SWAP, hex("42"), 2),
			shouldError: true,
			expectedErr: avm.ErrUnexpectedInstructionParameter,
		},
		{
			name:        "POP with unexpected param",
			instruction: avm.MakeInstructionWithParam(avm.POP, 1),
			shouldError: true,
			expectedErr: avm.ErrUnexpectedInstructionParameter,
		},
		{
			name:        "valid simple instruction",
			instruction: avm.MakeInstruction(avm.DUP),
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.instruction.ValidateInstruction()

			if tt.shouldError {
				if err == nil {
					t.Error("Expected validation error but got none")
				} else if tt.expectedErr != nil && err != tt.expectedErr {
					t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestValidationPreventsExecution(t *testing.T) {
	// Test that validation catches issues before execution starts
	tests := []struct {
		name         string
		instructions []avm.Instruction
		expectedErr  error
	}{
		{
			name: "negative value caught before execution",
			instructions: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(-2)), // First instruction fails
				avm.MakeInstruction(avm.EMIT),
			},
			expectedErr: avm.ErrNegativeValue,
		},
		{
			name: "invalid swap parameter caught after valid push",
			instructions: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex("1")), // Valid - consumes gas
				avm.MakeInstructionWithParam(avm.SWAP, 2000),     // Invalid - fails validation
			},
			expectedErr: avm.ErrInvalidSwapParameter,
		},
		{
			name: "first instruction invalid - no gas consumed",
			instructions: []avm.Instruction{
				avm.MakeInstructionWithParam(avm.SWAP, 2000), // First instruction fails
			},
			expectedErr: avm.ErrInvalidSwapParameter,
		},
		{
			name: "unexpected parameter caught",
			instructions: []avm.Instruction{
				avm.MakeInstructionWithParam(avm.POP, 1), // POP shouldn't have param
			},
			expectedErr: avm.ErrUnexpectedInstructionParameter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := avm.NewRuntime(1000)
			gasUsed, err := r.StartExecution(tt.instructions)

			// Should fail with validation error
			if err == nil {
				t.Error("Expected validation error but execution succeeded")
			}

			if err != tt.expectedErr {
				t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
			}

			// Check gas consumption based on test case
			switch tt.name {
			case "negative value caught before execution", "first instruction invalid - no gas consumed", "unexpected parameter caught":
				// First instruction should fail validation before consuming gas
				if gasUsed != 0 {
					t.Errorf("Expected 0 gas consumed (first instruction invalid), got %d", gasUsed)
				}
			case "invalid swap parameter caught after valid push":
				// PUSH should consume 3 gas before SWAP fails
				if gasUsed != 3 {
					t.Errorf("Expected 3 gas consumed (PUSH succeeded), got %d", gasUsed)
				}
			default:
				t.Logf("Gas consumed before validation failure: %d", gasUsed)
			}
		})
	}
}