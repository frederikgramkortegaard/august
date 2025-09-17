package tests

import (
	"august/marigold"
	"testing"
)

func TestStringIndexing(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "basic string indexing",
			input: `define test() : string {
				message: string = "hello"
				first: string = message[0]
				return first
			}`,
		},
		{
			name: "string indexing with expression",
			input: `define test() : string {
				text: string = "world"
				i: int = 2
				char: string = text[i]
				return char
			}`,
		},
		{
			name: "string indexing in expression",
			input: `define test() : string {
				word: string = "test"
				result: string = word[0] + word[1]
				return result
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

			// Typecheck should pass
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Typecheck failed: %v", r)
				}
			}()

			marigold.Typecheck(ast)
		})
	}
}

func TestStringConcatenation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "basic concatenation",
			input: `define test() : string {
				first: string = "hello"
				second: string = " world"
				result: string = first + second
				return result
			}`,
		},
		{
			name: "multiple concatenation",
			input: `define test() : string {
				a: string = "a"
				b: string = "b"
				c: string = "c"
				result: string = a + b + c
				return result
			}`,
		},
		{
			name: "literal concatenation",
			input: `define test() : string {
				result: string = "hello" + " " + "world"
				return result
			}`,
		},
		{
			name: "concatenation with indexing",
			input: `define test() : string {
				word: string = "test"
				result: string = word[0] + word[1] + word[2]
				return result
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

			// Typecheck should pass
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Typecheck failed: %v", r)
				}
			}()

			marigold.Typecheck(ast)
		})
	}
}

func TestStringLen(t *testing.T) {
	input := `define test() : int {
		message: string = "hello world"
		length: int = len(message)
		return length
	}`

	tokens, err := marigold.Lex(input)
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Typecheck should pass
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Typecheck failed: %v", r)
		}
	}()

	marigold.Typecheck(ast)

	// Verify the len() call is in the AST
	fn := ast.Functions["test"]
	if fn == nil {
		t.Fatalf("Function 'test' not found")
	}

	if len(fn.Block.Statements) < 2 {
		t.Fatalf("Expected at least 2 statements")
	}

	// Second statement should be the len() call
	lenStmt := fn.Block.Statements[1]
	if lenStmt.Type != marigold.AssignmentStmt {
		t.Fatalf("Expected assignment statement")
	}

	if lenStmt.Rhs == nil || lenStmt.Rhs.Type != marigold.CallExpr {
		t.Fatalf("Expected function call on RHS")
	}

	if lenStmt.Rhs.Value != "len" {
		t.Fatalf("Expected len() call, got %v", lenStmt.Rhs.Value)
	}
}

func TestStringErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		{
			name: "string index must be int",
			input: `define test() : string {
				text: string = "hello"
				char: string = text["invalid"]
				return char
			}`,
			shouldError: true,
		},
		{
			name: "cannot add string and int",
			input: `define test() : string {
				text: string = "hello"
				result: string = text + 42
				return result
			}`,
			shouldError: true,
		},
		{
			name: "cannot subtract strings",
			input: `define test() : string {
				a: string = "hello"
				b: string = "world"
				result: string = a - b
				return result
			}`,
			shouldError: true,
		},
		{
			name: "len on invalid type",
			input: `define test() : int {
				x: bool = true
				length: int = len(x)
				return length
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

func TestComplexStringOperations(t *testing.T) {
	input := `define test() : string {
		base: string = "hello"
		space: string = " "
		world: string = "world"

		// Get first character of each word
		h: string = base[0]
		w: string = world[0]

		// Calculate lengths
		base_len: int = len(base)
		total_len: int = len(base + space + world)

		// Build result
		result: string = h + w + space + "len="
		return result
	}`

	tokens, err := marigold.Lex(input)
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Typecheck should pass
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Typecheck failed: %v", r)
		}
	}()

	marigold.Typecheck(ast)

	// Verify we have the expected variables
	fn := ast.Functions["test"]
	if fn == nil {
		t.Fatalf("Function 'test' not found")
	}

	expectedVars := []string{"base", "space", "world", "h", "w", "base_len", "total_len", "result"}
	for _, varName := range expectedVars {
		if _, exists := fn.Block.Scope.Variables[varName]; !exists {
			t.Errorf("Expected variable '%s' not found in scope", varName)
		}
	}
}