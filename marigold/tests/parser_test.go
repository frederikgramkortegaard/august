package tests

import (
	"august/marigold"
	"strings"
	"testing"
)

func TestParserOnlyFunctionsInGlobalScope(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldPanic bool
	}{
		{
			name:        "Function definition is allowed",
			input:       `define foo() : int { return 1 }`,
			shouldPanic: false,
		},
		{
			name:        "Multiple functions are allowed",
			input:       `define foo() : int { return 1 } define bar() : int { return 2 }`,
			shouldPanic: false,
		},
		{
			name:        "Global variable assignment not allowed",
			input:       `x: int = 5`,
			shouldPanic: true,
		},
		{
			name:        "Global return statement not allowed",
			input:       `return 5`,
			shouldPanic: true,
		},
		{
			name:        "Global if statement not allowed",
			input:       `if true { emit "hello" }`,
			shouldPanic: true,
		},
		{
			name:        "Global while loop not allowed",
			input:       `while true { emit "loop" }`,
			shouldPanic: true,
		},
		{
			name:        "Global emit not allowed",
			input:       `emit "hello"`,
			shouldPanic: true,
		},
		{
			name:        "Global expression not allowed",
			input:       `1 + 2`,
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("Test panicked unexpectedly: %v", r)
					}
					// Expected panic
				} else if tt.shouldPanic {
					t.Errorf("Expected panic but didn't get one")
				}
			}()

			// Lex the input
			tokens, err := marigold.Lex(tt.input)
			if err != nil {
				t.Fatalf("Lex error: %v", err)
			}

			// Parse should panic for non-function global statements
			_, err = marigold.Parse(tokens)
			if err != nil && !tt.shouldPanic {
				t.Errorf("Parse error when none expected: %v", err)
			}
		})
	}
}

func TestParserFunctionBodies(t *testing.T) {
	// Functions can contain any statements in their bodies
	validPrograms := []string{
		`define main() : int {
			x: int = 5
			y: int = 10
			return x + y
		}`,
		`define loop() : int {
			i: int = 0
			while i < 10 {
				emit(i)
				i = i + 1
			}
			return 0
		}`,
		`define conditional() : int {
			if true {
				return 1
			} else {
				return 0
			}
		}`,
	}

	for i, program := range validPrograms {
		t.Run(strings.Split(program, "\n")[0], func(t *testing.T) {
			tokens, err := marigold.Lex(program)
			if err != nil {
				t.Fatalf("Test %d: Lex error: %v", i, err)
			}

			_, err = marigold.Parse(tokens)
			if err != nil {
				t.Errorf("Test %d: Parse error for valid program: %v", i, err)
			}
		})
	}
}