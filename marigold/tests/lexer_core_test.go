package tests

import (
	"august/marigold"
	"testing"
)

func TestLexBasicTokens(t *testing.T) {
	input := "()[]{},.:|;"
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := []struct {
		typ   marigold.TokenType
		value string
		col   uint64
	}{
		{marigold.LParen, "(", 0},
		{marigold.RParen, ")", 1},
		{marigold.LSquare, "[", 2},
		{marigold.RSquare, "]", 3},
		{marigold.LBrace, "{", 4},
		{marigold.RBrace, "}", 5},
		{marigold.Comma, ",", 6},
		{marigold.Dot, ".", 7},
		{marigold.Colon, ":", 8},
		{marigold.Pipe, "|", 9},
		{marigold.Semicolon, ";", 10},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, token := range tokens {
		if token.Type != expected[i].typ {
			t.Errorf("Token %d type: expected %s, got %s", i, expected[i].typ, token.Type)
		}
		if token.Value != expected[i].value {
			t.Errorf("Token %d value: expected %s, got %s", i, expected[i].value, token.Value)
		}
		if token.Col != expected[i].col {
			t.Errorf("Token %d col: expected %d, got %d", i, expected[i].col, token.Col)
		}
		if token.Row != 0 {
			t.Errorf("Token %d row: expected 0, got %d", i, token.Row)
		}
		if token.Filename != "dummy" {
			t.Errorf("Token %d filename: expected 'dummy', got %s", i, token.Filename)
		}
	}
}

func TestLexOperators(t *testing.T) {
	input := "= == != < <= > >= && ||"
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := []struct {
		typ   marigold.TokenType
		value string
	}{
		{marigold.Assign, "="},
		{marigold.Equal, "=="},
		{marigold.NotEqual, "!="},
		{marigold.LessThan, "<"},
		{marigold.LessEqual, "<="},
		{marigold.GreaterThan, ">"},
		{marigold.GreaterEqual, ">="},
		{marigold.LogicalAnd, "&&"},
		{marigold.LogicalOr, "||"},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, token := range tokens {
		if token.Type != expected[i].typ {
			t.Errorf("Token %d type: expected %s, got %s", i, expected[i].typ, token.Type)
		}
		if token.Value != expected[i].value {
			t.Errorf("Token %d value: expected %s, got %s", i, expected[i].value, token.Value)
		}
	}
}

func TestLexStringLiteral(t *testing.T) {
	input := `"hello world"`
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(tokens) != 1 {
		t.Fatalf("Expected 1 token, got %d", len(tokens))
	}

	token := tokens[0]
	if token.Type != marigold.StringLiteral {
		t.Errorf("Expected StringLiteral, got %s", token.Type)
	}
	if token.Value != "hello world" {
		t.Errorf("Expected 'hello world', got %s", token.Value)
	}
	if token.Col != 0 {
		t.Errorf("Expected col 0, got %d", token.Col)
	}
}

func TestLexComments(t *testing.T) {
	input := "( // this is a comment\n)"
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d", len(tokens))
	}

	// First token: (
	if tokens[0].Type != marigold.LParen {
		t.Errorf("First token type: expected LParen, got %s", tokens[0].Type)
	}
	if tokens[0].Value != "(" {
		t.Errorf("First token value: expected '(', got %s", tokens[0].Value)
	}
	if tokens[0].Row != 0 || tokens[0].Col != 0 {
		t.Errorf("First token position: expected (0,0), got (%d,%d)", tokens[0].Row, tokens[0].Col)
	}

	// Second token: )
	if tokens[1].Type != marigold.RParen {
		t.Errorf("Second token type: expected RParen, got %s", tokens[1].Type)
	}
	if tokens[1].Value != ")" {
		t.Errorf("Second token value: expected ')', got %s", tokens[1].Value)
	}
	if tokens[1].Row != 1 || tokens[1].Col != 0 {
		t.Errorf("Second token position: expected (1,0), got (%d,%d)", tokens[1].Row, tokens[1].Col)
	}
}

func TestUnterminatedString(t *testing.T) {
	input := `"unterminated`
	_, err := marigold.Lex(input)

	if err != marigold.ErrUnterminatedStringLiteral {
		t.Errorf("Expected ErrUnterminatedStringLiteral, got %v", err)
	}
}

func TestEmptyInput(t *testing.T) {
	input := ""
	_, err := marigold.Lex(input)

	if err != marigold.ErrInvalidSourceCode {
		t.Errorf("Expected ErrInvalidSourceCode, got %v", err)
	}
}

func TestLexBooleanLiterals(t *testing.T) {
	input := "true false"
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := []struct {
		typ   marigold.TokenType
		value string
	}{
		{marigold.BoolLiteral, "true"},
		{marigold.BoolLiteral, "false"},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, token := range tokens {
		if token.Type != expected[i].typ {
			t.Errorf("Token %d type: expected %s, got %s", i, expected[i].typ, token.Type)
		}
		if token.Value != expected[i].value {
			t.Errorf("Token %d value: expected %s, got %s", i, expected[i].value, token.Value)
		}
	}
}
