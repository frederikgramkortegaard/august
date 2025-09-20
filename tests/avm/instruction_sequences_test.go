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
		avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(2)), // Stack: [A, B, C, D, 2]
		avm.MakeInstruction(avm.SWAP),                    // Stack: [A, D, C, B] (swap top with 2nd from top)
	}

	r := avm.NewTestRuntime(1000, instructions)
	gasUsed, err := r.StartExecution()

	if err != nil {
		t.Fatalf("Execution error: %v", err)
	}

	// Verify gas consumption
	expectedGas := uint64(3*5 + 3) // 5 PUSH operations (3 gas each) + 1 SWAP (3 gas)
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

	r := avm.NewTestRuntime(100, instructions)
	gasUsed, err := r.StartExecution()

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
				avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(3)), // push swap index
				avm.MakeInstruction(avm.SWAP), // trying to swap 3 down with only 2 elements
			},
			shouldError: true,
		},
		{
			name: "swap 0 (should be valid)",
			setup: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex("1")),
				avm.MakeInstructionWithValue(avm.PUSH, hex("2")),
				avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(0)), // push swap index
				avm.MakeInstruction(avm.SWAP), // swap with self (no-op)
			},
			shouldError: false,
		},
		{
			name: "swap 1 (swap top two)",
			setup: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex("1")),
				avm.MakeInstructionWithValue(avm.PUSH, hex("2")),
				avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(1)), // push swap index
				avm.MakeInstruction(avm.SWAP), // swap top two elements
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := avm.NewTestRuntime(1000, tt.setup)

			// Execute all instructions including the swap
			_, err := r.StartExecution()

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
		avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(1)), // 3 gas (push swap index)
		avm.MakeInstruction(avm.SWAP),        // 3 gas
		avm.MakeInstruction(avm.POP),                     // 2 gas
		// Total: 17 gas
	}

	r := avm.NewTestRuntime(20, instructions) // Enough gas
	gasUsed, err := r.StartExecution()

	if err != nil {
		t.Fatalf("Execution error: %v", err)
	}

	expectedGas := uint64(17)
	if gasUsed != expectedGas {
		t.Errorf("Expected gas %d, got %d", expectedGas, gasUsed)
	}

	if r.GasAvailable != 3 { // 20 - 17 = 3
		t.Errorf("Expected remaining gas 3, got %d", r.GasAvailable)
	}
}

func TestOutOfGas(t *testing.T) {
	// Test running out of gas
	instructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, hex("1")), // 3 gas
		avm.MakeInstructionWithValue(avm.PUSH, hex("2")), // 3 gas
		avm.MakeInstructionWithValue(avm.PUSH, hex("3")), // 3 gas - this should fail
	}

	r := avm.NewTestRuntime(5, instructions) // Not enough gas (need 9)
	_, err := r.StartExecution()

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
			instruction: avm.MakeInstruction(avm.SWAP), // Note: this needs 2 pushed to stack first
			shouldError: false,
		},
		{
			name:        "valid SWAP with large param",
			instruction: avm.MakeInstruction(avm.SWAP), // Note: this needs 2000 pushed to stack first
			shouldError: false,
		},
		{
			name:        "SWAP with unexpected value",
			instruction: avm.MakeInstructionWithValueAndParam(avm.SWAP, hex("42"), 2),
			shouldError: true,
			expectedErr: avm.ErrUnexpectedInstructionParameter,
		},
		{
			name:        "valid POP",
			instruction: avm.MakeInstruction(avm.POP), // POP doesn't take parameters
			shouldError: false,
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
			name: "invalid swap parameter caught during execution",
			instructions: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex("1")), // Valid - consumes gas
				avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(2000)), // push swap index
				avm.MakeInstruction(avm.SWAP),     // Invalid - fails during execution
			},
			expectedErr: avm.ErrInvalidSwapParameter,
		},
		{
			name: "first instruction invalid - no gas consumed",
			instructions: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(-1)), // First instruction fails validation
				avm.MakeInstruction(avm.SWAP),
			},
			expectedErr: avm.ErrNegativeValue,
		},
		{
			name: "unexpected parameter caught",
			instructions: []avm.Instruction{
				avm.MakeInstructionWithValue(avm.POP, big.NewInt(42)), // POP shouldn't have param
			},
			expectedErr: avm.ErrUnexpectedInstructionParameter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := avm.NewTestRuntime(1000, tt.instructions)
			gasUsed, err := r.StartExecution()

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
			case "invalid swap parameter caught during execution":
				// PUSH (3) + PUSH (3) + SWAP (3) = 9 gas, even though SWAP fails during execution
				if gasUsed != 9 {
					t.Errorf("Expected 9 gas consumed (all instructions attempted), got %d", gasUsed)
				}
			default:
				t.Logf("Gas consumed before validation failure: %d", gasUsed)
			}
		})
	}
}

func TestArithmeticOperations(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string // hex values
		opcode   avm.OPCODE
		expected string // hex result
	}{
		{"ADD: 5 + 3", "5", "3", avm.ADD, "8"},
		{"SUB: 10 - 3", "A", "3", avm.SUB, "7"},
		{"MUL: 6 * 7", "6", "7", avm.MUL, "2A"}, // 42 in hex
		{"DIV: 20 / 4", "14", "4", avm.DIV, "5"},
		{"AND: 0xFF & 0x0F", "FF", "0F", avm.AND, "0F"},
		{"OR: 0xF0 | 0x0F", "F0", "0F", avm.OR, "FF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instructions := []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex(tt.a)),
				avm.MakeInstructionWithValue(avm.PUSH, hex(tt.b)),
				avm.MakeInstruction(tt.opcode),
			}

			r := avm.NewTestRuntime(100, instructions)
			_, err := r.StartExecution()
			if err != nil {
				t.Fatalf("Execution error: %v", err)
			}

			result, err := r.Stack.Pop()
			if err != nil {
				t.Fatalf("Failed to pop result: %v", err)
			}

			expected := hex(tt.expected)
			if result.Cmp(expected) != 0 {
				t.Errorf("Expected %s, got %s", expected.Text(16), result.Text(16))
			}
		})
	}
}

func TestComparisonOperations(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string // hex values
		opcode   avm.OPCODE
		expected int64 // 1 for true, 0 for false
	}{
		{"EQ: 5 == 5", "5", "5", avm.EQ, 1},
		{"EQ: 5 == 3", "5", "3", avm.EQ, 0},
		{"LT: 3 < 5", "3", "5", avm.LT, 1},
		{"LT: 5 < 3", "5", "3", avm.LT, 0},
		{"GT: 5 > 3", "5", "3", avm.GT, 1},
		{"GT: 3 > 5", "3", "5", avm.GT, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instructions := []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex(tt.a)),
				avm.MakeInstructionWithValue(avm.PUSH, hex(tt.b)),
				avm.MakeInstruction(tt.opcode),
			}

			r := avm.NewTestRuntime(100, instructions)
			_, err := r.StartExecution()
			if err != nil {
				t.Fatalf("Execution error: %v", err)
			}

			result, err := r.Stack.Pop()
			if err != nil {
				t.Fatalf("Failed to pop result: %v", err)
			}

			expected := big.NewInt(tt.expected)
			if result.Cmp(expected) != 0 {
				t.Errorf("Expected %d, got %s", tt.expected, result.Text(10))
			}
		})
	}
}

func TestIsZeroOperation(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected int64
	}{
		{"ISZERO: 0", "0", 1},
		{"ISZERO: 1", "1", 0},
		{"ISZERO: 42", "2A", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instructions := []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, hex(tt.value)),
				avm.MakeInstruction(avm.ISZERO),
			}

			r := avm.NewTestRuntime(50, instructions)
			_, err := r.StartExecution()
			if err != nil {
				t.Fatalf("Execution error: %v", err)
			}

			result, err := r.Stack.Pop()
			if err != nil {
				t.Fatalf("Failed to pop result: %v", err)
			}

			expected := big.NewInt(tt.expected)
			if result.Cmp(expected) != 0 {
				t.Errorf("Expected %d, got %s", tt.expected, result.Text(10))
			}
		})
	}
}

func TestJumpOperations(t *testing.T) {
	t.Run("JUMP: unconditional jump", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // 0: Push A
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(4)), // push jump target
			avm.MakeInstruction(avm.JUMP),        // 2: Jump to instruction 4
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // 3: This should be skipped
			avm.MakeInstructionWithValue(avm.PUSH, hex("C")), // 4: Push C
		}

		r := avm.NewTestRuntime(50, instructions)
		_, err := r.StartExecution()
		if err != nil {
			t.Fatalf("Execution error: %v", err)
		}

		// Stack should contain [A, C] from bottom to top
		// Pop in reverse order
		val1, err := r.Stack.Pop() // Should be C
		if err != nil {
			t.Fatalf("Failed to pop first value: %v", err)
		}
		if val1.Cmp(hex("C")) != 0 {
			t.Errorf("Expected C, got %s", val1.Text(16))
		}

		val2, err := r.Stack.Pop() // Should be A
		if err != nil {
			t.Fatalf("Failed to pop second value: %v", err)
		}
		if val2.Cmp(hex("A")) != 0 {
			t.Errorf("Expected A, got %s", val2.Text(16))
		}

		// Stack should be empty
		if r.Stack.Size() != 0 {
			t.Errorf("Expected empty stack, size is %d", r.Stack.Size())
		}
	})

	t.Run("JUMPC: conditional jump with true condition", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // 0: Push A
			avm.MakeInstructionWithValue(avm.PUSH, hex("1")), // 1: Push 1 (true condition)
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(6)), // push jump target
			avm.MakeInstruction(avm.JUMPC),       // 3: Jump to instruction 6 if top is 1
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // 4: This should be skipped
			avm.MakeInstructionWithValue(avm.PUSH, hex("C")), // 5: This should be skipped
			avm.MakeInstructionWithValue(avm.PUSH, hex("D")), // 6: Push D
		}

		r := avm.NewTestRuntime(100, instructions)
		_, err := r.StartExecution()
		if err != nil {
			t.Fatalf("Execution error: %v", err)
		}

		// Stack should contain [A, D] from bottom to top
		val1, err := r.Stack.Pop() // Should be D
		if err != nil {
			t.Fatalf("Failed to pop first value: %v", err)
		}
		if val1.Cmp(hex("D")) != 0 {
			t.Errorf("Expected D, got %s", val1.Text(16))
		}

		val2, err := r.Stack.Pop() // Should be A
		if err != nil {
			t.Fatalf("Failed to pop second value: %v", err)
		}
		if val2.Cmp(hex("A")) != 0 {
			t.Errorf("Expected A, got %s", val2.Text(16))
		}
	})

	t.Run("JUMPC: conditional jump with false condition", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // 0: Push A
			avm.MakeInstructionWithValue(avm.PUSH, hex("0")), // 1: Push 0 (false condition)
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(5)), // push jump target
			avm.MakeInstruction(avm.JUMPC),       // 2: Should not jump
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // 3: Push B
			avm.MakeInstructionWithValue(avm.PUSH, hex("C")), // 4: Push C
			avm.MakeInstructionWithValue(avm.PUSH, hex("D")), // 5: Push D
		}

		r := avm.NewTestRuntime(100, instructions)
		_, err := r.StartExecution()
		if err != nil {
			t.Fatalf("Execution error: %v", err)
		}

		// Stack should contain [A, B, C, D] from bottom to top
		expected := []string{"D", "C", "B", "A"}
		for i, exp := range expected {
			val, err := r.Stack.Pop()
			if err != nil {
				t.Fatalf("Failed to pop value %d: %v", i, err)
			}
			if val.Cmp(hex(exp)) != 0 {
				t.Errorf("Position %d: expected %s, got %s", i, exp, val.Text(16))
			}
		}
	})

	t.Run("JUMP: invalid destination", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // 0: Push A
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(10)), // push jump target
			avm.MakeInstruction(avm.JUMP),       // 1: Jump to invalid instruction 10
		}

		r := avm.NewTestRuntime(50, instructions)
		_, err := r.StartExecution()
		if err == nil {
			t.Fatal("Expected error for invalid jump destination")
		}

		// Should be ErrInvalidJump
		if err.Error() != "invalid jump destination" {
			t.Errorf("Expected 'invalid jump destination', got '%s'", err.Error())
		}
	})

	t.Run("JUMPC: invalid destination", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("1")), // 0: Push 1 (true condition)
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(10)), // push jump target
			avm.MakeInstruction(avm.JUMPC),      // 1: Jump to invalid instruction 10
		}

		r := avm.NewTestRuntime(50, instructions)
		_, err := r.StartExecution()
		if err == nil {
			t.Fatal("Expected error for invalid jump destination")
		}

		// Should be ErrInvalidJump
		if err.Error() != "invalid jump destination" {
			t.Errorf("Expected 'invalid jump destination', got '%s'", err.Error())
		}
	})

	t.Run("JUMP: jump backwards (simple loop prevention)", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // 0: Push A
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // 1: Push B
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(0)), // 2: Push jump target 0
			avm.MakeInstruction(avm.JUMP),                    // 3: Jump back to instruction 0
		}

		r := avm.NewTestRuntime(50, instructions)
		_, err := r.StartExecution()

		// This should fail due to out of gas (infinite loop)
		if err == nil {
			t.Fatal("Expected out of gas error for infinite loop")
		}

		if err.Error() != "out of gas" {
			t.Errorf("Expected 'out of gas', got '%s'", err.Error())
		}
	})
}

func TestStopOperation(t *testing.T) {
	t.Run("STOP: terminates execution early", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // 0: Push A
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // 1: Push B
			avm.MakeInstruction(avm.STOP),                    // 2: Stop execution
			avm.MakeInstructionWithValue(avm.PUSH, hex("C")), // 3: This should not execute
			avm.MakeInstructionWithValue(avm.PUSH, hex("D")), // 4: This should not execute
		}

		r := avm.NewTestRuntime(100, instructions)
		_, err := r.StartExecution()

		// Should get program stopped error
		if err == nil {
			t.Fatal("Expected program stopped error")
		}

		if err.Error() != "program stopped" {
			t.Errorf("Expected 'program stopped', got '%s'", err.Error())
		}

		// Stack should contain only A and B
		if r.Stack.Size() != 2 {
			t.Errorf("Expected stack size 2, got %d", r.Stack.Size())
		}

		// Check stack contents (pop in reverse order)
		val1, err := r.Stack.Pop() // Should be B
		if err != nil {
			t.Fatalf("Failed to pop first value: %v", err)
		}
		if val1.Cmp(hex("B")) != 0 {
			t.Errorf("Expected B, got %s", val1.Text(16))
		}

		val2, err := r.Stack.Pop() // Should be A
		if err != nil {
			t.Fatalf("Failed to pop second value: %v", err)
		}
		if val2.Cmp(hex("A")) != 0 {
			t.Errorf("Expected A, got %s", val2.Text(16))
		}
	})

	t.Run("STOP: as first instruction", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstruction(avm.STOP),                    // 0: Stop immediately
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // 1: This should not execute
		}

		r := avm.NewTestRuntime(50, instructions)
		gasUsed, err := r.StartExecution()

		// Should get program stopped error
		if err == nil {
			t.Fatal("Expected program stopped error")
		}

		if err.Error() != "program stopped" {
			t.Errorf("Expected 'program stopped', got '%s'", err.Error())
		}

		// Should have used gas for STOP instruction only
		expectedGas := uint64(0) // STOP costs 0 gas
		if gasUsed != expectedGas {
			t.Errorf("Expected gas %d, got %d", expectedGas, gasUsed)
		}

		// Stack should be empty
		if r.Stack.Size() != 0 {
			t.Errorf("Expected empty stack, got size %d", r.Stack.Size())
		}
	})

	t.Run("STOP: after some operations", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("5")), // 0: Push 5
			avm.MakeInstructionWithValue(avm.PUSH, hex("3")), // 1: Push 3
			avm.MakeInstruction(avm.ADD),                     // 2: 5 + 3 = 8
			avm.MakeInstruction(avm.STOP),                    // 3: Stop here
			avm.MakeInstructionWithValue(avm.PUSH, hex("9")), // 4: Should not execute
		}

		r := avm.NewTestRuntime(100, instructions)
		gasUsed, err := r.StartExecution()

		// Should get program stopped error
		if err == nil {
			t.Fatal("Expected program stopped error")
		}

		if err.Error() != "program stopped" {
			t.Errorf("Expected 'program stopped', got '%s'", err.Error())
		}

		// Should have used gas for PUSH + PUSH + ADD + STOP = 3 + 3 + 3 + 0 = 9
		expectedGas := uint64(9)
		if gasUsed != expectedGas {
			t.Errorf("Expected gas %d, got %d", expectedGas, gasUsed)
		}

		// Stack should contain the result of ADD (8)
		if r.Stack.Size() != 1 {
			t.Errorf("Expected stack size 1, got %d", r.Stack.Size())
		}

		val, err := r.Stack.Pop()
		if err != nil {
			t.Fatalf("Failed to pop result: %v", err)
		}
		if val.Cmp(hex("8")) != 0 {
			t.Errorf("Expected 8, got %s", val.Text(16))
		}
	})
}

func TestStackOverflow(t *testing.T) {
	t.Run("Stack overflow with small stack size", func(t *testing.T) {
		// Test with small stack - runtime config not implemented yet
		// TODO: Implement stack size limits in RuntimeConfig

		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // 0: Push A (stack: [A])
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // 1: Push B (stack: [A, B]) - should be at limit
			avm.MakeInstructionWithValue(avm.PUSH, hex("C")), // 2: Push C - should cause overflow
		}

		r := avm.NewTestRuntimeWithLimits(100, instructions, 2, 1024) // Stack size 2
		_, err := r.StartExecution()

		// Should get stack overflow error
		if err == nil {
			t.Fatal("Expected stack overflow error")
		}

		if err.Error() != "stack overflow" {
			t.Errorf("Expected 'stack overflow', got '%s'", err.Error())
		}

		// Stack should contain A and B
		if r.Stack.Size() != 2 {
			t.Errorf("Expected stack size 2, got %d", r.Stack.Size())
		}
	})

	t.Run("No overflow with unlimited stack", func(t *testing.T) {
		// Config with unlimited stack size not implemented yet
		// TODO: Implement unlimited stack configuration

		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")),
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")),
			avm.MakeInstructionWithValue(avm.PUSH, hex("C")),
			avm.MakeInstructionWithValue(avm.PUSH, hex("D")),
			avm.MakeInstructionWithValue(avm.PUSH, hex("E")),
		}

		r := avm.NewTestRuntime(100, instructions)
		_, err := r.StartExecution()

		// Should succeed
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Stack should contain all 5 elements
		if r.Stack.Size() != 5 {
			t.Errorf("Expected stack size 5, got %d", r.Stack.Size())
		}
	})

	t.Run("DUP causes overflow", func(t *testing.T) {
		// Test that DUP can also cause overflow
		// TODO: Implement stack size limits for DUP operations

		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // 0: Push A (stack: [A]) - at limit
			avm.MakeInstruction(avm.DUP),                     // 1: DUP - should cause overflow
		}

		r := avm.NewTestRuntimeWithLimits(100, instructions, 1, 1024) // Stack size 1
		_, err := r.StartExecution()

		// Should get stack overflow error
		if err == nil {
			t.Fatal("Expected stack overflow error")
		}

		if err.Error() != "stack overflow" {
			t.Errorf("Expected 'stack overflow', got '%s'", err.Error())
		}

		// Stack should contain only A
		if r.Stack.Size() != 1 {
			t.Errorf("Expected stack size 1, got %d", r.Stack.Size())
		}
	})
}

func TestMemoryOperations(t *testing.T) {
	t.Run("Basic MSTORE and MLOAD", func(t *testing.T) {
		// Store value 42 at address 0, then load it back
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("0")),  // address 0
			avm.MakeInstructionWithValue(avm.PUSH, hex("2A")), // value 42
			avm.MakeInstruction(avm.MSTORE),                   // Store 42 at address 0
			avm.MakeInstructionWithValue(avm.PUSH, hex("0")),  // address 0
			avm.MakeInstruction(avm.MLOAD),                    // Load from address 0
		}

		r := avm.NewTestRuntime(1000, instructions)
		gasUsed, err := r.StartExecution()

		if err != nil {
			t.Fatalf("Execution error: %v", err)
		}

		// Verify gas consumption: 2 PUSH (3 each) + MSTORE (20) + PUSH (3) + MLOAD (15) = 44
		expectedGas := uint64(3 + 3 + 20 + 3 + 15)
		if gasUsed != expectedGas {
			t.Errorf("Expected gas %d, got %d", expectedGas, gasUsed)
		}

		// Verify the loaded value
		val, err := r.Stack.Pop()
		if err != nil {
			t.Fatalf("Failed to pop from stack: %v", err)
		}

		if val.Cmp(hex("2A")) != 0 {
			t.Errorf("Expected value 42, got %s", val.String())
		}
	})

	t.Run("Multiple memory locations", func(t *testing.T) {
		// Store different values at different addresses
		instructions := []avm.Instruction{
			// Store 100 at address 5
			avm.MakeInstructionWithValue(avm.PUSH, hex("5")),  // address 5
			avm.MakeInstructionWithValue(avm.PUSH, hex("64")), // value 100
			avm.MakeInstruction(avm.MSTORE),
			// Store 200 at address 10
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")),  // address 10
			avm.MakeInstructionWithValue(avm.PUSH, hex("C8")), // value 200
			avm.MakeInstruction(avm.MSTORE),
			// Load from address 5
			avm.MakeInstructionWithValue(avm.PUSH, hex("5")),
			avm.MakeInstruction(avm.MLOAD),
			// Load from address 10
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")),
			avm.MakeInstruction(avm.MLOAD),
		}

		r := avm.NewTestRuntime(1000, instructions)
		_, err := r.StartExecution()

		if err != nil {
			t.Fatalf("Execution error: %v", err)
		}

		// Stack should have [100, 200] from bottom to top
		val2, err := r.Stack.Pop() // Should be 200
		if err != nil {
			t.Fatalf("Failed to pop from stack: %v", err)
		}
		if val2.Cmp(hex("C8")) != 0 {
			t.Errorf("Expected value 200, got %s", val2.String())
		}

		val1, err := r.Stack.Pop() // Should be 100
		if err != nil {
			t.Fatalf("Failed to pop from stack: %v", err)
		}
		if val1.Cmp(hex("64")) != 0 {
			t.Errorf("Expected value 100, got %s", val1.String())
		}
	})

	t.Run("Load from uninitialized memory", func(t *testing.T) {
		// Load from address that was never written to
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("FF")), // address 255
			avm.MakeInstruction(avm.MLOAD),                    // Load from uninitialized address
		}

		r := avm.NewTestRuntime(1000, instructions)
		_, err := r.StartExecution()

		if err != nil {
			t.Fatalf("Execution error: %v", err)
		}

		// Should return 0 for uninitialized memory
		val, err := r.Stack.Pop()
		if err != nil {
			t.Fatalf("Failed to pop from stack: %v", err)
		}

		if val.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("Expected value 0 for uninitialized memory, got %s", val.String())
		}
	})

	t.Run("Memory out of bounds - store", func(t *testing.T) {
		// Try to store beyond memory limit
		// TODO: Implement memory limits in RuntimeConfig

		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")),  // address 10 (out of bounds for size 10)
			avm.MakeInstructionWithValue(avm.PUSH, hex("64")), // value 100
			avm.MakeInstruction(avm.MSTORE),
		}

		r := avm.NewTestRuntimeWithLimits(1000, instructions, 1024, 10) // Memory size 10
		_, err := r.StartExecution()

		if err == nil {
			t.Fatal("Expected memory out of bounds error, got nil")
		}

		if err.Error() != "memory access out of bounds" {
			t.Errorf("Expected 'memory access out of bounds', got '%s'", err.Error())
		}
	})

	t.Run("Memory out of bounds - load", func(t *testing.T) {
		// Try to load beyond memory limit
		// TODO: Implement memory limits in RuntimeConfig

		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // address 10 (out of bounds for size 10)
			avm.MakeInstruction(avm.MLOAD),
		}

		r := avm.NewTestRuntimeWithLimits(1000, instructions, 1024, 10) // Memory size 10
		_, err := r.StartExecution()

		if err == nil {
			t.Fatal("Expected memory out of bounds error, got nil")
		}

		if err.Error() != "memory access out of bounds" {
			t.Errorf("Expected 'memory access out of bounds', got '%s'", err.Error())
		}
	})

	t.Run("Unlimited memory", func(t *testing.T) {
		// Test with unlimited memory
		// TODO: Implement unlimited memory configuration

		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("FFFF")), // Large address
			avm.MakeInstructionWithValue(avm.PUSH, hex("42")),   // value 66
			avm.MakeInstruction(avm.MSTORE),
			avm.MakeInstructionWithValue(avm.PUSH, hex("FFFF")),
			avm.MakeInstruction(avm.MLOAD),
		}

		r := avm.NewTestRuntimeWithLimits(1000, instructions, 1024, 0) // Unlimited memory
		_, err := r.StartExecution()

		if err != nil {
			t.Fatalf("Execution error with unlimited memory: %v", err)
		}

		// Verify the loaded value
		val, err := r.Stack.Pop()
		if err != nil {
			t.Fatalf("Failed to pop from stack: %v", err)
		}

		if val.Cmp(hex("42")) != 0 {
			t.Errorf("Expected value 66, got %s", val.String())
		}
	})

	t.Run("Stack underflow on MSTORE", func(t *testing.T) {
		// Try MSTORE with insufficient stack items
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("0")), // Only one value on stack
			avm.MakeInstruction(avm.MSTORE),                  // Needs two values: address and value
		}

		r := avm.NewTestRuntime(1000, instructions)
		_, err := r.StartExecution()

		if err == nil {
			t.Fatal("Expected stack underflow error, got nil")
		}

		if err.Error() != "Empty Stack" {
			t.Errorf("Expected 'Empty Stack', got '%s'", err.Error())
		}
	})

	t.Run("Stack underflow on MLOAD", func(t *testing.T) {
		// Try MLOAD with empty stack
		instructions := []avm.Instruction{
			avm.MakeInstruction(avm.MLOAD), // Needs address on stack
		}

		r := avm.NewTestRuntime(1000, instructions)
		_, err := r.StartExecution()

		if err == nil {
			t.Fatal("Expected stack underflow error, got nil")
		}

		if err.Error() != "Empty Stack" {
			t.Errorf("Expected 'Empty Stack', got '%s'", err.Error())
		}
	})
}

func TestSwapValidationBasedOnCurrentStackSize(t *testing.T) {
	t.Run("SWAP param valid for current stack size", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // Stack: [A]
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // Stack: [A, B]
			avm.MakeInstructionWithValue(avm.PUSH, hex("C")), // Stack: [A, B, C]
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(2)), // Stack: [A, B, C, 2]
			avm.MakeInstruction(avm.SWAP),                    // Valid: swap with 2nd from top (stack has 3 elements)
		}

		r := avm.NewTestRuntime(100, instructions)
		_, err := r.StartExecution()

		if err != nil {
			t.Errorf("Expected success but got error: %v", err)
		}
	})

	t.Run("SWAP param invalid for current stack size", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // Stack: [A]
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // Stack: [A, B]
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(2)), // Stack: [A, B, 2]
			avm.MakeInstruction(avm.SWAP),                    // Invalid: trying to swap with 2nd from top when stack only has 2 elements (indexes 0,1)
		}

		r := avm.NewTestRuntime(100, instructions)
		_, err := r.StartExecution()

		if err == nil {
			t.Error("Expected ErrInvalidSwapParameter but got no error")
		}

		if err != avm.ErrInvalidSwapParameter {
			t.Errorf("Expected ErrInvalidSwapParameter, got %v", err)
		}
	})

	t.Run("SWAP param at boundary of current stack size", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // Stack: [A]
			avm.MakeInstructionWithValue(avm.PUSH, hex("B")), // Stack: [A, B]
			avm.MakeInstructionWithValue(avm.PUSH, hex("C")), // Stack: [A, B, C]
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(3)), // Stack: [A, B, C, 3]
			avm.MakeInstruction(avm.SWAP),                    // Invalid: param equals stack size (indexes 0,1,2 but trying to access 3)
		}

		r := avm.NewTestRuntime(100, instructions)
		_, err := r.StartExecution()

		if err == nil {
			t.Error("Expected ErrInvalidSwapParameter but got no error")
		}

		if err != avm.ErrInvalidSwapParameter {
			t.Errorf("Expected ErrInvalidSwapParameter, got %v", err)
		}
	})

	t.Run("SWAP param 0 always valid with non-empty stack", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, hex("A")), // Stack: [A]
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(0)), // Stack: [A, 0]
			avm.MakeInstruction(avm.SWAP),                    // Valid: swap with self
		}

		r := avm.NewTestRuntime(100, instructions)
		_, err := r.StartExecution()

		if err != nil {
			t.Errorf("Expected success but got error: %v", err)
		}
	})

	t.Run("SWAP with empty stack", func(t *testing.T) {
		instructions := []avm.Instruction{
			avm.MakeInstructionWithValue(avm.PUSH, big.NewInt(0)), // Stack: [0]
			avm.MakeInstruction(avm.SWAP), // Invalid: no elements on stack to swap with
		}

		r := avm.NewTestRuntime(100, instructions)
		_, err := r.StartExecution()

		if err == nil {
			t.Error("Expected ErrInvalidSwapParameter but got no error")
		}

		if err != avm.ErrInvalidSwapParameter {
			t.Errorf("Expected ErrInvalidSwapParameter, got %v", err)
		}
	})
}