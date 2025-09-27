package tests

import (
	"august/marigold"
	"testing"
)

func TestDanglingExpressions(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		shouldFail bool
	}{
		{
			name: "standalone number expression",
			input: `define test() : int {
				42
				return 0
			}`,
			shouldFail: false,
		},
		{
			name: "standalone variable expression",
			input: `define test() : int {
				x
				return 0
			}`,
			shouldFail: false,
		},
		{
			name: "standalone arithmetic expression",
			input: `define test() : int {
				x + y * 2
				return 0
			}`,
			shouldFail: false,
		},
		{
			name: "standalone function call expression",
			input: `define test() : int {
				someFunc(1, 2)
				return 0
			}`,
			shouldFail: false,
		},
		{
			name: "standalone parenthesized expression",
			input: `define test() : int {
				(x + y)
				return 0
			}`,
			shouldFail: false,
		},
		{
			name: "multiple dangling expressions",
			input: `define test() : int {
				42
				x + y
				func(1)
				return 0
			}`,
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := marigold.Lex(tt.input)
			if err != nil {
				t.Fatalf("Lex error: %v", err)
			}

			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldFail {
						t.Errorf("Parse panicked unexpectedly: %v", r)
					}
				}
			}()

			ast, err := marigold.Parse(tokens)
			if tt.shouldFail {
				if err == nil {
					t.Errorf("Expected parse to fail, but it succeeded")
				}
			} else {
				if err != nil {
					t.Errorf("Parse error: %v", err)
				}
				if ast == nil {
					t.Errorf("AST is nil")
				}
				if len(ast.Functions) != 1 {
					t.Errorf("Expected 1 function, got %d", len(ast.Functions))
				}
			}
		})
	}
}

func TestInvalidTokensInStatements(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "random operator in statement position",
			input: `define test() : int {
				+
				return 0
			}`,
		},
		{
			name: "comma in statement position",
			input: `define test() : int {
				,
				return 0
			}`,
		},
		{
			name: "colon in statement position",
			input: `define test() : int {
				:
				return 0
			}`,
		},
		{
			name: "right paren in statement position",
			input: `define test() : int {
				)
				return 0
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := marigold.Lex(tt.input)
			if err != nil {
				t.Fatalf("Lex error: %v", err)
			}

			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected parse to panic on invalid token, but it didn't")
				}
			}()

			marigold.Parse(tokens)
		})
	}
}
