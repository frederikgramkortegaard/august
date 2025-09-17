package tests

import (
	"august/marigold"
	"testing"
)

func TestMixedArrayMapOperations(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		description string
	}{
		{
			name: "array of maps",
			input: `define test() : int {
				maps: []map[string]int = []
				return 0
			}`,
			shouldError: false,
			description: "Should support arrays containing maps",
		},
		{
			name: "map with array values",
			input: `define test() : int {
				data: map[string][]int = {}
				data["numbers"] = [1, 2, 3]
				return len(data["numbers"])
			}`,
			shouldError: false,
			description: "Should support maps with array values",
		},
		{
			name: "len on map values that are arrays",
			input: `define test() : int {
				data: map[string][]string = {}
				data["words"] = ["hello", "world"]
				count: int = len(data["words"])
				return count
			}`,
			shouldError: false,
			description: "Should support len() on array values from maps",
		},
		{
			name: "array indexing with map lookup",
			input: `define test() : string {
				indices: map[string]int = {}
				words: []string = ["zero", "one", "two"]
				indices["second"] = 2
				return words[indices["second"]]
			}`,
			shouldError: false,
			description: "Should support using map values as array indices",
		},
		{
			name: "string indexing with arithmetic",
			input: `define test() : string {
				text: string = "hello"
				pos: int = 1 + 2
				return text[pos]
			}`,
			shouldError: false,
			description: "Should support string indexing with arithmetic expressions",
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

func TestComplexTypeConversions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		description string
	}{
		{
			name: "float division in array index",
			input: `define test() : int {
				numbers: [5]int = [10, 20, 30, 40, 50]
				index: int = 8 / 2
				return numbers[index]
			}`,
			shouldError: false,
			description: "Should allow float result of division to be used as int index",
		},
		{
			name: "mixed arithmetic in string concatenation",
			input: `define test() : string {
				base: string = "result: "
				value: float = 3.14
				count: int = 5
				combined: float = value + count
				return base + "mixed"
			}`,
			shouldError: false,
			description: "Should handle mixed arithmetic alongside string operations",
		},
		{
			name: "map with computed keys",
			input: `define test() : int {
				data: map[string]int = {}
				prefix: string = "key"
				suffix: string = "1"
				key: string = prefix + suffix
				data[key] = 42
				return data["key1"]
			}`,
			shouldError: false,
			description: "Should support computed string keys for maps",
		},
		{
			name: "len comparison with arithmetic",
			input: `define test() : bool {
				text: string = "hello"
				numbers: [3]int = [1, 2, 3]
				return len(text) > len(numbers)
			}`,
			shouldError: false,
			description: "Should support comparing len() results",
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

func TestTypeErrorEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		description string
	}{
		{
			name: "cannot index int with array",
			input: `define test() : int {
				number: int = 42
				indices: [1]int = [0]
				return number[indices[0]]
			}`,
			shouldError: true,
			description: "Should not allow indexing integers",
		},
		{
			name: "cannot use string as array index",
			input: `define test() : int {
				numbers: [3]int = [1, 2, 3]
				key: string = "not_a_number"
				return numbers[key]
			}`,
			shouldError: true,
			description: "Should not allow string as array index",
		},
		{
			name: "cannot assign array to map",
			input: `define test() : int {
				data: map[string]int = [1, 2, 3]
				return 0
			}`,
			shouldError: true,
			description: "Should not allow assigning array literal to map variable",
		},
		{
			name: "cannot len on bool",
			input: `define test() : int {
				flag: bool = true
				return len(flag)
			}`,
			shouldError: true,
			description: "Should not allow len() on boolean values",
		},
		{
			name: "cannot reassign different map type",
			input: `define test() : int {
				data: map[string]int = {}
				data = {}
				return 0
			}`,
			shouldError: true,
			description: "Should not allow reassigning map with ambiguous empty literal",
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