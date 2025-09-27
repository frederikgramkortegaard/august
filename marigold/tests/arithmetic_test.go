package tests

import (
	"august/marigold"
	"testing"
)

func TestArithmeticTypePromotion(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "int + int stays int",
			input: `define test() : int {
				a: int = 5
				b: int = 3
				result: int = a + b
				return result
			}`,
		},
		{
			name: "int + float becomes float",
			input: `define test() : float {
				a: int = 5
				b: float = 3.5
				result: float = a + b
				return result
			}`,
		},
		{
			name: "float + int becomes float",
			input: `define test() : float {
				a: float = 5.2
				b: int = 3
				result: float = a + b
				return result
			}`,
		},
		{
			name: "assign int to float variable",
			input: `define test() : float {
				a: int = 5
				result: float = a
				return result
			}`,
		},
		{
			name: "int * float becomes float",
			input: `define test() : float {
				a: int = 4
				b: float = 2.5
				result: float = a * b
				return result
			}`,
		},
		{
			name: "int - float becomes float",
			input: `define test() : float {
				a: int = 10
				b: float = 3.2
				result: float = a - b
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

func TestDivisionAlwaysReturnsFloat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "int / int returns float",
			input: `define test() : float {
				a: int = 10
				b: int = 3
				result: float = a / b
				return result
			}`,
		},
		{
			name: "float / float returns float",
			input: `define test() : float {
				a: float = 10.0
				b: float = 3.0
				result: float = a / b
				return result
			}`,
		},
		{
			name: "int / float returns float",
			input: `define test() : float {
				a: int = 10
				b: float = 3.0
				result: float = a / b
				return result
			}`,
		},
		{
			name: "float / int returns float",
			input: `define test() : float {
				a: float = 10.0
				b: int = 3
				result: float = a / b
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

func TestFloatToIntConversion(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "assign float result to int variable",
			input: `define test() : int {
				a: int = 10
				b: int = 3
				result: int = a / b
				return result
			}`,
		},
		{
			name: "return float from int function",
			input: `define test() : int {
				a: float = 5.7
				b: float = 2.1
				return a + b
			}`,
		},
		{
			name: "use float as array index",
			input: `define test() : int {
				arr: [3]int = [1, 2, 3]
				index: float = 1.9
				result: int = arr[index]
				return result
			}`,
		},
		{
			name: "use float as string index",
			input: `define test() : string {
				text: string = "hello"
				index: float = 2.8
				char: string = text[index]
				return char
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

func TestMixedArithmeticComparison(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "compare int and float",
			input: `define test() : bool {
				a: int = 5
				b: float = 5.0
				result: bool = a == b
				return result
			}`,
		},
		{
			name: "compare with arithmetic result",
			input: `define test() : bool {
				a: int = 10
				b: float = 3.0
				result: bool = a / b > 3.0
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

func TestArithmeticErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		{
			name: "cannot add string and int",
			input: `define test() : string {
				a: string = "hello"
				b: int = 5
				result: string = a + b
				return result
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
