package tests

import (
	"august/marigold"
	"testing"
)

func TestMapDeclaration(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "empty map declaration",
			input: `define test() : map[string]int {
				balances: map[string]int = {}
				return balances
			}`,
		},
		{
			name: "map parameter",
			input: `define process(data: map[string]float) : string {
				return "processed"
			}`,
		},
		{
			name: "map return type",
			input: `define get_scores() : map[string]int {
				scores: map[string]int = {}
				return scores
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

func TestMapIndexing(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "map value assignment",
			input: `define test() : int {
				balances: map[string]int = {}
				balances["alice"] = 100
				balances["bob"] = 50
				return 0
			}`,
		},
		{
			name: "map value access",
			input: `define test() : int {
				balances: map[string]int = {}
				balances["alice"] = 100
				amount: int = balances["alice"]
				return amount
			}`,
		},
		{
			name: "map with parameter",
			input: `define get_score(scores: map[string]float, user: string) : float {
				score: float = scores[user]
				return score
			}`,
		},
		{
			name: "complex map operations",
			input: `define test() : int {
				balances: map[string]int = {}
				users: [2]string = ["alice", "bob"]

				balances[users[0]] = 100
				balances[users[1]] = 50

				total: int = balances["alice"] + balances["bob"]
				return total
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

func TestMapTypeConversions(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "float to int map assignment",
			input: `define test() : int {
				scores: map[string]int = {}
				scores["user"] = 95.7
				result: int = scores["user"]
				return result
			}`,
		},
		{
			name: "int to float map assignment",
			input: `define test() : float {
				scores: map[string]float = {}
				scores["user"] = 95
				result: float = scores["user"]
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

func TestMapErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		{
			name: "map key must be string",
			input: `define test() : int {
				balances: map[string]int = {}
				balances[42] = 100
				return 0
			}`,
			shouldError: true,
		},
		{
			name: "wrong value type assignment",
			input: `define test() : int {
				balances: map[string]int = {}
				balances["alice"] = "invalid"
				return 0
			}`,
			shouldError: true,
		},
		{
			name: "cannot use int as map key",
			input: `define test() : int {
				balances: map[string]int = {}
				key: int = 1
				amount: int = balances[key]
				return amount
			}`,
			shouldError: true,
		},
		{
			name: "cannot assign wrong type to map variable",
			input: `define test() : int {
				balances: map[string]int = "invalid"
				return 0
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

func TestMapWithStringTypes(t *testing.T) {
	input := `define test() : string {
		names: map[string]string = {}
		names["first"] = "Alice"
		names["last"] = "Smith"

		full_name: string = names["first"] + " " + names["last"]
		return full_name
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

	// Verify the map variable is in the AST
	fn := ast.Functions["test"]
	if fn == nil {
		t.Fatalf("Function 'test' not found")
	}

	if _, exists := fn.Block.Scope.Variables["names"]; !exists {
		t.Errorf("Expected variable 'names' not found in scope")
	}
}
