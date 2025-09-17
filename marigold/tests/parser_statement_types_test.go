package tests

import (
	"august/marigold"
	"testing"
)

func TestStatementTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected marigold.StatementType
	}{
		{
			name: "assignment statement",
			input: `define test() : int {
				x = 5
				return 0
			}`,
			expected: marigold.AssignmentStmt,
		},
		{
			name: "variable declaration statement",
			input: `define test() : int {
				x: int = 5
				return 0
			}`,
			expected: marigold.AssignmentStmt,
		},
		{
			name: "return statement",
			input: `define test() : int {
				return 42
			}`,
			expected: marigold.ReturnStmt,
		},
		{
			name: "if statement",
			input: `define test() : int {
				if x > 0 {
					return 1
				}
				return 0
			}`,
			expected: marigold.IfStmt,
		},
		{
			name: "while statement",
			input: `define test() : int {
				while x > 0 {
					x = x - 1
				}
				return 0
			}`,
			expected: marigold.WhileStmt,
		},
		{
			name: "emit statement",
			input: `define test() : int {
				emit 42
				return 0
			}`,
			expected: marigold.EmitStmt,
		},
		{
			name: "expression statement - function call",
			input: `define test() : int {
				func(1, 2)
				return 0
			}`,
			expected: marigold.ExpressionStmt,
		},
		{
			name: "expression statement - arithmetic",
			input: `define test() : int {
				x + y
				return 0
			}`,
			expected: marigold.ExpressionStmt,
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
			if ast == nil || len(ast.Functions) == 0 {
				t.Fatalf("No functions parsed")
			}

			fn := ast.Functions["test"]
			if fn.Block == nil || len(fn.Block.Statements) == 0 {
				t.Fatalf("No statements in function")
			}

			stmt := fn.Block.Statements[0]
			if stmt.Type != tt.expected {
				t.Errorf("Expected statement type %s, got %s", tt.expected, stmt.Type)
			}
		})
	}
}

func TestIfElseStatements(t *testing.T) {
	input := `define test() : int {
		if x > 0 {
			return 1
		} else {
			return 0
		}
	}`

	tokens, err := marigold.Lex(input)
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Errorf("Parse error: %v", err)
	}

	fn := ast.Functions["test"]
	ifStmt := fn.Block.Statements[0]

	if ifStmt.Type != marigold.IfStmt {
		t.Errorf("Expected marigold.IfStmt, got %s", ifStmt.Type)
	}

	if ifStmt.Block == nil {
		t.Errorf("If block is nil")
	}

	if ifStmt.ElseBlock == nil {
		t.Errorf("Else block is nil")
	}

	if len(ifStmt.Block.Statements) != 1 {
		t.Errorf("Expected 1 statement in if block, got %d", len(ifStmt.Block.Statements))
	}

	if len(ifStmt.ElseBlock.Statements) != 1 {
		t.Errorf("Expected 1 statement in else block, got %d", len(ifStmt.ElseBlock.Statements))
	}
}

func TestVariableDeclarationTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected marigold.TokenType
	}{
		{
			name: "int variable",
			input: `define test() : int {
				x: int = 5
				return 0
			}`,
			expected: marigold.Int,
		},
		{
			name: "float variable",
			input: `define test() : int {
				x: float = 3.14
				return 0
			}`,
			expected: marigold.Float,
		},
		{
			name: "string variable",
			input: `define test() : int {
				x: string = "hello"
				return 0
			}`,
			expected: marigold.String,
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

			fn := ast.Functions["test"]
			stmt := fn.Block.Statements[0]

			if stmt.Type != marigold.AssignmentStmt {
				t.Errorf("Expected marigold.AssignmentStmt, got %s", stmt.Type)
			}

			if stmt.VarType != tt.expected {
				t.Errorf("Expected variable type %s, got %s", tt.expected, stmt.VarType)
			}
		})
	}
}