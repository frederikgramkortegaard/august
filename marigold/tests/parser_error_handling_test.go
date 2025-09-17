package tests

import (
	"august/marigold"
	"testing"
)

func TestParserErrorHandling(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "missing function body",
			input: `define test() : int`,
		},
		{
			name: "missing return type",
			input: `define test() {
				return 0
			}`,
		},
		{
			name: "missing parameter type",
			input: `define test(x) : int {
				return 0
			}`,
		},
		{
			name: "invalid parameter type",
			input: `define test(x: invalid) : int {
				return 0
			}`,
		},
		{
			name: "invalid return type",
			input: `define test() : invalid {
				return 0
			}`,
		},
		{
			name: "missing assignment value",
			input: `define test() : int {
				x: int =
				return 0
			}`,
		},
		{
			name: "missing variable type after colon",
			input: `define test() : int {
				x: = 5
				return 0
			}`,
		},
		{
			name: "missing condition in if statement",
			input: `define test() : int {
				if {
					return 1
				}
				return 0
			}`,
		},
		{
			name: "missing condition in while statement",
			input: `define test() : int {
				while {
					return 1
				}
				return 0
			}`,
		},
		{
			name: "missing expression after emit",
			input: `define test() : int {
				emit
				return 0
			}`,
		},
		{
			name: "missing expression after return",
			input: `define test() : int {
				return
			}`,
		},
		{
			name: "unclosed parentheses in expression",
			input: `define test() : int {
				x = (1 + 2
				return 0
			}`,
		},
		{
			name: "unclosed function call",
			input: `define test() : int {
				func(1, 2
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
					t.Errorf("Expected parse to panic on malformed input, but it didn't")
				}
			}()

			marigold.Parse(tokens)
		})
	}
}

func TestComplexExpressionStatements(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "nested function calls",
			input: `define test() : int {
				func1(func2(1), func3(2, 3))
				return 0
			}`,
		},
		{
			name: "complex arithmetic expressions",
			input: `define test() : int {
				x + y * z - w / v
				return 0
			}`,
		},
		{
			name: "parenthesized expressions",
			input: `define test() : int {
				(x + y) * (z - w)
				return 0
			}`,
		},
		{
			name: "mixed expressions and statements",
			input: `define test() : int {
				x: int = 5
				y: int = 10
				x + y
				func(x, y)
				if x > y {
					return x
				}
				return y
			}`,
		},
		{
			name: "unary expressions",
			input: `define test() : int {
				-x
				-func(1)
				-(x + y)
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

			ast, err := marigold.Parse(tokens)
			if err != nil {
				t.Errorf("Parse error: %v", err)
			}
			if ast == nil {
				t.Errorf("AST is nil")
			}
			if len(ast.Functions) != 1 {
				t.Errorf("Expected 1 function, got %d", len(ast.Functions))
			}
		})
	}
}