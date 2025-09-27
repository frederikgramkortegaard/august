package tests

import (
	"august/marigold"
	"testing"
)

func TestLexKeywords(t *testing.T) {
	input := "if else while return define emit"
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := []struct {
		typ   marigold.TokenType
		value string
	}{
		{marigold.If, "if"},
		{marigold.Else, "else"},
		{marigold.While, "while"},
		{marigold.Return, "return"},
		{marigold.Define, "define"},
		{marigold.Emit, "emit"},
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

func TestLexTypeKeywords(t *testing.T) {
	input := "string int float"
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := []struct {
		typ   marigold.TokenType
		value string
	}{
		{marigold.String, "string"},
		{marigold.Int, "int"},
		{marigold.Float, "float"},
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

func TestLexIdentifiers(t *testing.T) {
	input := "myVar anotherVar camelCase"
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := []struct {
		typ   marigold.TokenType
		value string
	}{
		{marigold.Identifier, "myVar"},
		{marigold.Identifier, "anotherVar"},
		{marigold.Identifier, "camelCase"},
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

func TestLexNumbers(t *testing.T) {
	input := "123 456.789"
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := []struct {
		typ   marigold.TokenType
		value string
	}{
		{marigold.IntLiteral, "123"},
		{marigold.FloatLiteral, "456.789"},
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

func TestLexComplexExpression(t *testing.T) {
	input := `define add(x: int, y: int) { return x + y; }`
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Just verify we get the expected number of tokens for major parts
	if len(tokens) < 10 {
		t.Fatalf("Expected at least 10 tokens, got %d", len(tokens))
	}

	// Check first few tokens
	if tokens[0].Type != marigold.Define || tokens[0].Value != "define" {
		t.Errorf("First token: expected Define 'define', got %s '%s'", tokens[0].Type, tokens[0].Value)
	}
	if tokens[1].Type != marigold.Identifier || tokens[1].Value != "add" {
		t.Errorf("Second token: expected Identifier 'add', got %s '%s'", tokens[1].Type, tokens[1].Value)
	}
}
