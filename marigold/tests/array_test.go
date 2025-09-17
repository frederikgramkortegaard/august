package tests

import (
	"august/marigold"
	"testing"
)

func TestArrayTypes(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "explicit size array",
			input: `define test() : int {
				numbers: [5]int = [1, 2, 3, 4, 5]
				return numbers[0]
			}`,
		},
		{
			name: "inferred size array",
			input: `define test() : int {
				numbers: []int = [10, 20, 30]
				return numbers[1]
			}`,
		},
		{
			name: "uninitialized array",
			input: `define test() : int {
				numbers: [3]int
				return 0
			}`,
		},
		{
			name: "string array",
			input: `define test() : string {
				names: [2]string = ["alice", "bob"]
				return names[0]
			}`,
		},
		{
			name: "bool array",
			input: `define test() : bool {
				flags: []bool = [true, false, true]
				return flags[2]
			}`,
		},
		{
			name: "empty array",
			input: `define test() : int {
				empty: [0]int = []
				return 42
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := marigold.Lex(tt.input)
			if err != nil {
				t.Fatalf("Lex error: %v", err)
			}

			ast, err := marigold.Parse(tokens)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if ast == nil || len(ast.Functions) == 0 {
				t.Fatalf("No functions parsed")
			}

			// Just verify parsing succeeds - we can't run the code yet
			fn := ast.Functions["test"]
			if fn == nil {
				t.Fatalf("Function 'test' not found")
			}
		})
	}
}

func TestLenFunction(t *testing.T) {
	input := `define test() : int {
		numbers: [5]int = [1, 2, 3, 4, 5]
		size: int = len(numbers)
		return size
	}`

	tokens, err := marigold.Lex(input)
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if ast == nil {
		t.Fatalf("AST is nil")
	}

	fn := ast.Functions["test"]
	if fn == nil {
		t.Fatalf("Function 'test' not found")
	}

	// Verify we have the len() call in the AST
	if len(fn.Block.Statements) < 2 {
		t.Fatalf("Expected at least 2 statements in function")
	}

	// Second statement should be: size: int = len(numbers)
	sizeStmt := fn.Block.Statements[1]
	if sizeStmt.Type != marigold.AssignmentStmt {
		t.Fatalf("Expected assignment statement, got %s", sizeStmt.Type)
	}

	if sizeStmt.Rhs == nil || sizeStmt.Rhs.Type != marigold.CallExpr {
		t.Fatalf("Expected function call on RHS")
	}

	if sizeStmt.Rhs.Value != "len" {
		t.Fatalf("Expected len() call, got %v", sizeStmt.Rhs.Value)
	}
}

func TestArrayErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		{
			name: "size mismatch",
			input: `define test() : int {
				numbers: [3]int = [1, 2, 3, 4, 5]
				return 0
			}`,
			shouldError: true,
		},
		{
			name: "type mismatch",
			input: `define test() : int {
				numbers: [3]int = ["hello", "world", "test"]
				return 0
			}`,
			shouldError: true,
		},
		{
			name: "mixed element types",
			input: `define test() : int {
				mixed: [2]int = [1, "hello"]
				return 0
			}`,
			shouldError: true,
		},
		{
			name: "len on non-array",
			input: `define test() : int {
				x: int = 5
				size: int = len(x)
				return size
			}`,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := marigold.Lex(tt.input)
			if err != nil {
				if !tt.shouldError {
					t.Fatalf("Unexpected lex error: %v", err)
				}
				return
			}

			ast, err := marigold.Parse(tokens)
			if err != nil {
				if !tt.shouldError {
					t.Fatalf("Unexpected parse error: %v", err)
				}
				return
			}

			// Try typechecking
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldError {
						t.Errorf("Unexpected typecheck panic: %v", r)
					}
					// Expected error, test passes
				} else {
					if tt.shouldError {
						t.Errorf("Expected typecheck to fail but it succeeded")
					}
				}
			}()

			marigold.Typecheck(ast)
		})
	}
}

func TestArrayAssignmentRestrictions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		description string
	}{
		{
			name: "array index assignment allowed",
			input: `define test() : int {
				numbers: [3]int = [1, 2, 3]
				numbers[0] = 10
				return numbers[0]
			}`,
			shouldError: false,
			description: "Should allow assignment to array elements after declaration",
		},
		{
			name: "array whole reassignment forbidden - same size",
			input: `define test() : int {
				numbers: [3]int = [1, 2, 3]
				numbers = [4, 5, 6]
				return numbers[0]
			}`,
			shouldError: true,
			description: "Should NOT allow whole array reassignment even with same size",
		},
		{
			name: "array whole reassignment forbidden - different size",
			input: `define test() : int {
				numbers: [3]int = [1, 2, 3]
				numbers = [4, 5]
				return numbers[0]
			}`,
			shouldError: true,
			description: "Should NOT allow whole array reassignment with different size",
		},
		{
			name: "inferred array index assignment allowed",
			input: `define test() : string {
				names: []string = ["alice", "bob"]
				names[1] = "charlie"
				return names[1]
			}`,
			shouldError: false,
			description: "Should allow assignment to inferred array elements",
		},
		{
			name: "map assignment for comparison",
			input: `define test() : int {
				data: map[string]int = {}
				data["key"] = 42
				return data["key"]
			}`,
			shouldError: false,
			description: "Maps should still allow indexed assignment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := marigold.Lex(tt.input)
			if err != nil {
				if !tt.shouldError {
					t.Fatalf("Unexpected lex error: %v", err)
				}
				return
			}

			ast, err := marigold.Parse(tokens)
			if err != nil {
				if !tt.shouldError {
					t.Fatalf("Unexpected parse error: %v", err)
				}
				return
			}

			// Try typechecking
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldError {
						t.Errorf("Unexpected typecheck panic for %s: %v", tt.description, r)
					}
					// Expected error, test passes
				} else {
					if tt.shouldError {
						t.Errorf("Expected typecheck to fail for %s but it succeeded", tt.description)
					}
				}
			}()

			marigold.Typecheck(ast)
		})
	}
}

func TestLenKeywordShadowing(t *testing.T) {
	// Test that 'len' cannot be used as a function name
	input := `define len() : int {
		return 42
	}`

	tokens, err := marigold.Lex(input)
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	// This should fail at parse time because 'len' is now a keyword
	defer func() {
		if r := recover(); r != nil {
			// Expected panic due to parse error, test passes
			return
		}
		t.Fatalf("Expected parse to panic when defining function named 'len'")
	}()

	_, err = marigold.Parse(tokens)
	if err == nil {
		t.Fatalf("Expected parse error when defining function named 'len'")
	}
}