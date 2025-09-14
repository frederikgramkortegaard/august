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
		{Opcode: avm.PUSH, Rhs: hex("A")},    // Stack: [A]
		{Opcode: avm.PUSH, Rhs: hex("B")},    // Stack: [A, B]
		{Opcode: avm.PUSH, Rhs: hex("C")},    // Stack: [A, B, C]
		{Opcode: avm.PUSH, Rhs: hex("D")},    // Stack: [A, B, C, D]
		{Opcode: avm.SWAP, Rhs: hex("2")},    // Stack: [A, D, C, B] (swap top with 2nd from top)
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
		{Opcode: avm.PUSH, Rhs: hex("42")},   // Stack: [42]
		{Opcode: avm.DUP},                    // Stack: [42, 42]
		{Opcode: avm.PUSH, Rhs: hex("100")},  // Stack: [42, 42, 100]
		{Opcode: avm.POP},                    // Stack: [42, 42]
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
				{Opcode: avm.PUSH, Rhs: hex("1")},
				{Opcode: avm.PUSH, Rhs: hex("2")},
				{Opcode: avm.SWAP, Rhs: hex("3")}, // trying to swap 3 down with only 2 elements
			},
			shouldError: true,
		},
		{
			name: "swap 0 (should be valid)",
			setup: []avm.Instruction{
				{Opcode: avm.PUSH, Rhs: hex("1")},
				{Opcode: avm.PUSH, Rhs: hex("2")},
				{Opcode: avm.SWAP, Rhs: hex("0")}, // swap with self (no-op)
			},
			shouldError: false,
		},
		{
			name: "swap 1 (swap top two)",
			setup: []avm.Instruction{
				{Opcode: avm.PUSH, Rhs: hex("1")},
				{Opcode: avm.PUSH, Rhs: hex("2")},
				{Opcode: avm.SWAP, Rhs: hex("1")}, // swap top two elements
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
		{Opcode: avm.PUSH, Rhs: hex("1")}, // 3 gas
		{Opcode: avm.PUSH, Rhs: hex("2")}, // 3 gas
		{Opcode: avm.DUP},                 // 3 gas
		{Opcode: avm.SWAP, Rhs: hex("1")}, // 3 gas
		{Opcode: avm.POP},                 // 2 gas
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
		{Opcode: avm.PUSH, Rhs: hex("1")}, // 3 gas
		{Opcode: avm.PUSH, Rhs: hex("2")}, // 3 gas
		{Opcode: avm.PUSH, Rhs: hex("3")}, // 3 gas - this should fail
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