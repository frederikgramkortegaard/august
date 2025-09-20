package avm

import (
	"august/avm"
	"math/big"
	"testing"
)

func TestSignedArithmetic(t *testing.T) {
	t.Log("=== Testing Signed 256-bit Integer Arithmetic ===")

	// Test cases for signed arithmetic
	testCases := []struct {
		name     string
		a        string // hex representation
		b        string // hex representation
		op       string
		expected string // expected result in hex
	}{
		// Positive + Positive
		{"5 + 3", "0x05", "0x03", "ADD", "0x08"},

		// Positive + Negative (represented as two's complement)
		{"5 + (-1)", "0x05", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", "ADD", "0x04"},

		// Negative + Negative
		{"(-1) + (-1)", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", "ADD", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFE"},

		// Subtraction: Positive - Positive
		{"10 - 3", "0x0A", "0x03", "SUB", "0x07"},

		// Subtraction: Positive - Negative = Positive + Positive
		{"5 - (-3)", "0x05", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFD", "SUB", "0x08"},

		// Subtraction: Negative - Positive = Negative + Negative
		{"(-5) - 3", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFB", "0x03", "SUB", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF8"},

		// Multiplication
		{"3 * 4", "0x03", "0x04", "MUL", "0x0C"},
		{"(-3) * 4", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFD", "0x04", "MUL", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF4"},
		{"(-3) * (-4)", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFD", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFC", "MUL", "0x0C"},

		// Division
		{"12 / 3", "0x0C", "0x03", "DIV", "0x04"},
		{"(-12) / 3", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF4", "0x03", "DIV", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFC"},
		{"(-12) / (-3)", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF4", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFD", "DIV", "0x04"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse hex values to big.Int
			aBig := new(big.Int)
			aBig.SetString(tc.a, 0) // Auto-detect base
			bBig := new(big.Int)
			bBig.SetString(tc.b, 0) // Auto-detect base

			// Create AVM runtime
			instructions := []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, aBig), // Push first operand
				avm.MakeInstructionWithValue(avm.PUSH, bBig), // Push second operand
			}

			// Add the operation
			switch tc.op {
			case "ADD":
				instructions = append(instructions, avm.MakeInstruction(avm.ADD))
			case "SUB":
				instructions = append(instructions, avm.MakeInstruction(avm.SUB))
			case "MUL":
				instructions = append(instructions, avm.MakeInstruction(avm.MUL))
			case "DIV":
				instructions = append(instructions, avm.MakeInstruction(avm.DIV))
			}

			// Create mock blockchain context
			blockContext := &avm.BlockchainContext{
				BlockNumber:   1,
				GasLimit:      10000,
				ChainID:       1,
			}

			runtime := avm.NewRuntime(10000, instructions, blockContext)

			// Execute
			_, err := runtime.StartExecution()
			if err != nil && err != avm.ErrProgramStopped {
				t.Fatalf("Execution failed: %v", err)
			}

			// Check result
			if runtime.Stack.Size() != 1 {
				t.Fatalf("Expected 1 value on stack, got %d", runtime.Stack.Size())
			}

			result, _ := runtime.Stack.Pop()
			expectedBig := new(big.Int)
			expectedBig.SetString(tc.expected, 0) // Parse with auto-detect base

			if result.Cmp(expectedBig) != 0 {
				t.Errorf("Expected %s, got %s (hex: 0x%x)", tc.expected, result.String(), result)
			} else {
				t.Logf("✓ %s = %s (hex: 0x%x)", tc.name, result.String(), result)
			}
		})
	}
}

func TestSignedComparisons(t *testing.T) {
	t.Log("=== Testing Signed Comparisons ===")

	testCases := []struct {
		name     string
		a        string // hex representation
		b        string // hex representation
		op       string
		expected bool
	}{
		// Positive comparisons
		{"5 < 10", "0x05", "0x0A", "LT", true},
		{"10 < 5", "0x0A", "0x05", "LT", false},
		{"5 > 3", "0x05", "0x03", "GT", true},
		{"3 > 5", "0x03", "0x05", "GT", false},

		// Negative comparisons
		{"(-5) < (-3)", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFB", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFD", "LT", true},
		{"(-3) < (-5)", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFD", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFB", "LT", false},

		// Mixed positive/negative
		{"(-1) < 1", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", "0x01", "LT", true},
		{"1 > (-1)", "0x01", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", "GT", true},
		{"(-10) < 5", "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF6", "0x05", "LT", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse hex values to big.Int
			aBig := new(big.Int)
			aBig.SetString(tc.a, 0) // Auto-detect base
			bBig := new(big.Int)
			bBig.SetString(tc.b, 0) // Auto-detect base

			// Create AVM runtime
			instructions := []avm.Instruction{
				avm.MakeInstructionWithValue(avm.PUSH, aBig), // Push first operand
				avm.MakeInstructionWithValue(avm.PUSH, bBig), // Push second operand
			}

			// Add the comparison operation
			switch tc.op {
			case "LT":
				instructions = append(instructions, avm.MakeInstruction(avm.LT))
			case "GT":
				instructions = append(instructions, avm.MakeInstruction(avm.GT))
			case "EQ":
				instructions = append(instructions, avm.MakeInstruction(avm.EQ))
			}

			// Create mock blockchain context
			blockContext := &avm.BlockchainContext{
				BlockNumber:   1,
				GasLimit:      10000,
				ChainID:       1,
			}

			runtime := avm.NewRuntime(10000, instructions, blockContext)

			// Execute
			_, err := runtime.StartExecution()
			if err != nil && err != avm.ErrProgramStopped {
				t.Fatalf("Execution failed: %v", err)
			}

			// Check result
			if runtime.Stack.Size() != 1 {
				t.Fatalf("Expected 1 value on stack, got %d", runtime.Stack.Size())
			}

			result, _ := runtime.Stack.Pop()
			actualBool := result.Cmp(big.NewInt(0)) != 0

			if actualBool != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, actualBool)
			} else {
				t.Logf("✓ %s = %v", tc.name, actualBool)
			}
		})
	}
}