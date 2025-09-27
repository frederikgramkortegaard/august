package tests

import (
	"august/marigold"
	"testing"
)

func TestBlockchainContextVariables(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		description string
	}{
		{
			name: "access caller",
			input: `define main() : int {
				emit(@caller)
				return 0
			}`,
			shouldError: false,
			description: "Should allow accessing @caller variable",
		},
		{
			name: "access address",
			input: `define main() : int {
				emit(@address)
				return 0
			}`,
			shouldError: false,
			description: "Should allow accessing @address variable",
		},
		{
			name: "access balance",
			input: `define main() : int {
				return @balance
			}`,
			shouldError: false,
			description: "Should allow accessing @balance variable",
		},
		{
			name: "access origin",
			input: `define main() : int {
				emit(@origin)
				return 0
			}`,
			shouldError: false,
			description: "Should allow accessing @origin variable",
		},
		{
			name: "access gasprice",
			input: `define main() : int {
				return @gasprice
			}`,
			shouldError: false,
			description: "Should allow accessing @gasprice variable",
		},
		{
			name: "access callvalue",
			input: `define main() : int {
				return @callvalue
			}`,
			shouldError: false,
			description: "Should allow accessing @callvalue variable",
		},
		{
			name: "access timestamp",
			input: `define main() : int {
				return @timestamp
			}`,
			shouldError: false,
			description: "Should allow accessing @timestamp variable",
		},
		{
			name: "access difficulty",
			input: `define main() : int {
				return @difficulty
			}`,
			shouldError: false,
			description: "Should allow accessing @difficulty variable",
		},
		{
			name: "access coinbase",
			input: `define main() : int {
				emit(@coinbase)
				return 0
			}`,
			shouldError: false,
			description: "Should allow accessing @coinbase variable",
		},
		{
			name: "access height",
			input: `define main() : int {
				return @height
			}`,
			shouldError: false,
			description: "Should allow accessing @height variable",
		},
		{
			name: "access gaslimit",
			input: `define main() : int {
				return @gaslimit
			}`,
			shouldError: false,
			description: "Should allow accessing @gaslimit variable",
		},
		{
			name: "use in expressions",
			input: `define main() : int {
				return @height + @gasprice
			}`,
			shouldError: false,
			description: "Should allow using blockchain context variables in expressions",
		},
		{
			name: "emit blockchain context",
			input: `define main() : int {
				emit(@caller)
				emit(@height)
				return 0
			}`,
			shouldError: false,
			description: "Should allow emitting blockchain context variables",
		},
		{
			name: "wrong type access",
			input: `define main() : int {
				x: int = @caller
				return x
			}`,
			shouldError: true,
			description: "Should reject assigning string variable to int",
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
