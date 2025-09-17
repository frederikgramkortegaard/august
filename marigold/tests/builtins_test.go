package tests

import (
	"august/marigold"
	"testing"
)

func TestEmitFunction(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		description string
	}{
		{
			name: "emit int",
			input: `define test() : int {
				emit(42)
				return 0
			}`,
			shouldError: false,
			description: "Should allow emitting int values",
		},
		{
			name: "emit float",
			input: `define test() : int {
				emit(3.14)
				return 0
			}`,
			shouldError: false,
			description: "Should allow emitting float values",
		},
		{
			name: "emit string",
			input: `define test() : int {
				emit("hello")
				return 0
			}`,
			shouldError: false,
			description: "Should allow emitting string values",
		},
		{
			name: "emit bool",
			input: `define test() : int {
				emit(true)
				return 0
			}`,
			shouldError: false,
			description: "Should allow emitting boolean values",
		},
		{
			name: "emit wrong arg count",
			input: `define test() : int {
				emit(42, 99)
				return 0
			}`,
			shouldError: true,
			description: "Should reject emit with wrong number of arguments",
		},
		{
			name: "emit array error",
			input: `define test() : int {
				arr: [2]int = [1, 2]
				emit(arr)
				return 0
			}`,
			shouldError: true,
			description: "Should reject emitting complex types like arrays",
		},
		{
			name: "emit map error",
			input: `define test() : int {
				data: map[string]int = {}
				emit(data)
				return 0
			}`,
			shouldError: true,
			description: "Should reject emitting complex types like maps",
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

			// Test typechecking
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

func TestStopFunction(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		description string
	}{
		{
			name: "stop no args",
			input: `define test() : int {
				stop()
				return 0
			}`,
			shouldError: false,
			description: "Should allow stop with no arguments",
		},
		{
			name: "stop wrong arg count",
			input: `define test() : int {
				stop(42)
				return 0
			}`,
			shouldError: true,
			description: "Should reject stop with arguments",
		},
		{
			name: "stop in conditional",
			input: `define test() : int {
				if true {
					stop()
				}
				return 0
			}`,
			shouldError: false,
			description: "Should allow stop in conditional blocks",
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

			// Test typechecking
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

func TestBuiltinKeywordShadowing(t *testing.T) {
	// Test that emit and stop cannot be used as function names
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "cannot define emit function",
			input: `define emit() : int {
				return 42
			}`,
		},
		{
			name: "cannot define stop function",
			input: `define stop() : int {
				return 42
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := marigold.Lex(tt.input)
			if err != nil {
				// Expected to fail at lex or parse time
				return
			}

			defer func() {
				if r := recover(); r != nil {
					// Expected panic due to keyword shadowing
					return
				}
				t.Fatalf("Expected parse to panic when defining function with reserved name")
			}()

			_, err = marigold.Parse(tokens)
			if err == nil {
				t.Fatalf("Expected parse error when defining function with reserved name")
			}
		})
	}
}