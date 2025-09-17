package tests

import (
	"august/marigold"
	"strings"
	"testing"
)

func TestLexEOFEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldPanic bool
		expectError bool
	}{
		{
			name:        "EOF after opening quote",
			input:       `"unterminated`,
			shouldPanic: false,
			expectError: true,
		},
		{
			name:        "EOF after escape in string",
			input:       `"escape at end\`,
			shouldPanic: false,
			expectError: true,
		},
		{
			name:        "EOF after comment start",
			input:       `// comment with no newline`,
			shouldPanic: false,
			expectError: false,
		},
		{
			name:        "EOF after single slash",
			input:       `/`,
			shouldPanic: false,
			expectError: false,
		},
		{
			name:        "EOF after number dot",
			input:       `123.`,
			shouldPanic: false,
			expectError: false,
		},
		{
			name:        "EOF in middle of operator",
			input:       `=`,
			shouldPanic: false,
			expectError: false,
		},
		{
			name:        "EOF after equals waiting for second equals",
			input:       `= `,
			shouldPanic: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("Test panicked unexpectedly: %v", r)
					}
				}
			}()

			tokens, err := marigold.Lex(tt.input)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if !tt.expectError && err == nil {
				// If no error expected, we should get some tokens
				_ = tokens
			}
		})
	}
}

func TestLexInvalidCharacters(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldPanic bool
	}{
		{
			name:        "Unicode characters",
			input:       `α`,
			shouldPanic: true,
		},
		{
			name:        "Tab character",
			input:       "\t",
			shouldPanic: false, // Should be whitespace
		},
		{
			name:        "Null byte",
			input:       "\x00",
			shouldPanic: true,
		},
		{
			name:        "Control characters",
			input:       "\x01\x02\x03",
			shouldPanic: true,
		},
		{
			name:        "High ASCII",
			input:       "\xFF",
			shouldPanic: true,
		},
		{
			name:        "Emoji",
			input:       "🚀",
			shouldPanic: true,
		},
		{
			name:        "Unrecognized punctuation",
			input:       "@#$%^&",
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
					// Expected panic, test passes
				} else if tt.shouldPanic {
					t.Errorf("Expected panic but didn't get one")
				}
			}()

			_, err := marigold.Lex(tt.input)
			if err != nil && !tt.shouldPanic {
				t.Errorf("Got error when none expected: %v", err)
			}
		})
	}
}

func TestLexLargeInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Very long identifier",
			input: strings.Repeat("a", 10000),
		},
		{
			name:  "Very long string literal",
			input: `"` + strings.Repeat("x", 10000) + `"`,
		},
		{
			name:  "Very large number",
			input: strings.Repeat("9", 1000),
		},
		{
			name:  "Very long comment",
			input: "// " + strings.Repeat("comment ", 1000),
		},
		{
			name:  "Many tokens",
			input: strings.Repeat("( ) ", 10000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := marigold.Lex(tt.input)
			if err != nil {
				t.Errorf("Large input failed: %v", err)
			}
			if tokens == nil {
				t.Error("Got nil tokens")
			}
		})
	}
}

func TestLexDeepNesting(t *testing.T) {
	// Create deeply nested braces
	depth := 1000
	input := strings.Repeat("{", depth) + strings.Repeat("}", depth)

	tokens, err := marigold.Lex(input)
	if err != nil {
		t.Errorf("Deep nesting failed: %v", err)
	}

	if len(tokens) != depth*2 {
		t.Errorf("Expected %d tokens, got %d", depth*2, len(tokens))
	}
}

func TestLexMalformedNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // number of tokens expected
	}{
		{
			name:     "Multiple dots",
			input:    "1.2.3",
			expected: 3, // Should be: 1.2, ., 3
		},
		{
			name:     "Dot at end",
			input:    "123.",
			expected: 2, // Should be: 123, .
		},
		{
			name:     "Leading dot",
			input:    ".123",
			expected: 2, // Should be: ., 123
		},
		{
			name:     "Just dot",
			input:    ".",
			expected: 1, // Should be: .
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := marigold.Lex(tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(tokens) != tt.expected {
				t.Errorf("Expected %d tokens, got %d", tt.expected, len(tokens))
				for i, token := range tokens {
					t.Logf("Token %d: %s = %s", i, token.Type, token.Value)
				}
			}
		})
	}
}

func TestLexStringEscapeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expectedVal string
	}{
		{
			name:        "Escape at very end",
			input:       `"test\`,
			expectError: true,
		},
		{
			name:        "Double escape",
			input:       `"test\\"`,
			expectError: false,
			expectedVal: `test\`,
		},
		{
			name:        "Escape quote",
			input:       `"say \"hello\""`,
			expectError: false,
			expectedVal: `say "hello"`,
		},
		{
			name:        "Multiple escapes",
			input:       `"a\\b\\c"`,
			expectError: false,
			expectedVal: `a\b\c`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := marigold.Lex(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(tokens) != 1 {
				t.Errorf("Expected 1 token, got %d", len(tokens))
				return
			}

			if tokens[0].Type != marigold.StringLiteral {
				t.Errorf("Expected StringLiteral, got %s", tokens[0].Type)
			}

			if tokens[0].Value != tt.expectedVal {
				t.Errorf("Expected value %q, got %q", tt.expectedVal, tokens[0].Value)
			}
		})
	}
}

func TestLexPositionTracking(t *testing.T) {
	input := `line1
	line2
		line3`

	tokens, err := marigold.Lex(input)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have 3 identifier tokens
	if len(tokens) != 3 {
		t.Fatalf("Expected 3 tokens, got %d", len(tokens))
	}

	// Check positions
	expectedPositions := [][2]uint64{
		{0, 0}, // line1
		{1, 1}, // line2 (indented by 1 tab)
		{2, 2}, // line3 (indented by 2 tabs)
	}

	for i, token := range tokens {
		expectedRow, expectedCol := expectedPositions[i][0], expectedPositions[i][1]
		if token.Row != expectedRow || token.Col != expectedCol {
			t.Errorf("Token %d position: expected (%d,%d), got (%d,%d)",
				i, expectedRow, expectedCol, token.Row, token.Col)
		}
	}
}