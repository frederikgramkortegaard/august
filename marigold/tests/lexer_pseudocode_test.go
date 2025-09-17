package tests

import (
	"august/marigold"
	"testing"
)

func TestLexSimpleFunction(t *testing.T) {
	input := `define factorial(n: int) {
		if n <= 1 {
			return 1;
		} else {
			return n * factorial(n - 1);
		}
	}`
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check key tokens are present
	expectedTokens := []marigold.TokenType{
		marigold.Define,
		marigold.Identifier, // factorial
		marigold.LParen,
		marigold.Identifier, // n
		marigold.Colon,
		marigold.Int,
		marigold.RParen,
		marigold.LBrace,
		marigold.If,
		marigold.Identifier, // n
		marigold.LessEqual,
		marigold.IntLiteral, // 1
		marigold.LBrace,
		marigold.Return,
		marigold.IntLiteral, // 1
		marigold.Semicolon,
		marigold.RBrace,
		marigold.Else,
		marigold.LBrace,
		marigold.Return,
		marigold.Identifier, // n
		marigold.Multiply,
		marigold.Identifier, // factorial
		marigold.LParen,
		marigold.Identifier, // n
		marigold.Minus,
		marigold.IntLiteral, // 1
		marigold.RParen,
		marigold.Semicolon,
		marigold.RBrace,
		marigold.RBrace,
	}

	if len(tokens) < len(expectedTokens) {
		t.Fatalf("Expected at least %d tokens, got %d", len(expectedTokens), len(tokens))
	}

	// Verify first few important tokens
	for i, expected := range expectedTokens[:10] {
		if i >= len(tokens) {
			t.Fatalf("Not enough tokens, expected at least %d", i+1)
		}
		if tokens[i].Type != expected {
			t.Errorf("Token %d: expected %s, got %s (value: %s)", i, expected, tokens[i].Type, tokens[i].Value)
		}
	}
}

func TestLexVariableDeclarations(t *testing.T) {
	input := `define main() {
		name: string = "Alice";
		age: int = 25;
		height: float = 5.75;
		emit name;
		emit age + 1;
	}`
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Look for specific patterns
	foundStringDecl := false
	foundIntDecl := false
	foundFloatDecl := false
	foundEmit := false

	for i := 0; i < len(tokens)-2; i++ {
		// Look for "name: string"
		if tokens[i].Type == marigold.Identifier && tokens[i].Value == "name" &&
			i+2 < len(tokens) && tokens[i+1].Type == marigold.Colon && tokens[i+2].Type == marigold.String {
			foundStringDecl = true
		}
		// Look for "age: int"
		if tokens[i].Type == marigold.Identifier && tokens[i].Value == "age" &&
			i+2 < len(tokens) && tokens[i+1].Type == marigold.Colon && tokens[i+2].Type == marigold.Int {
			foundIntDecl = true
		}
		// Look for "height: float"
		if tokens[i].Type == marigold.Identifier && tokens[i].Value == "height" &&
			i+2 < len(tokens) && tokens[i+1].Type == marigold.Colon && tokens[i+2].Type == marigold.Float {
			foundFloatDecl = true
		}
		// Look for "emit"
		if tokens[i].Type == marigold.Emit {
			foundEmit = true
		}
	}

	if !foundStringDecl {
		t.Error("Expected to find string variable declaration")
	}
	if !foundIntDecl {
		t.Error("Expected to find int variable declaration")
	}
	if !foundFloatDecl {
		t.Error("Expected to find float variable declaration")
	}
	if !foundEmit {
		t.Error("Expected to find emit statement")
	}
}

func TestLexWhileLoop(t *testing.T) {
	input := `define countdown(n: int) {
		while n > 0 {
			emit n;
			n = n - 1;
		}
		emit "Done!";
	}`
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check that we have the while keyword and loop structure
	foundWhile := false
	foundGreaterThan := false
	foundStringLiteral := false

	for _, token := range tokens {
		if token.Type == marigold.While {
			foundWhile = true
		}
		if token.Type == marigold.GreaterThan {
			foundGreaterThan = true
		}
		if token.Type == marigold.StringLiteral && token.Value == "Done!" {
			foundStringLiteral = true
		}
	}

	if !foundWhile {
		t.Error("Expected to find while keyword")
	}
	if !foundGreaterThan {
		t.Error("Expected to find > operator")
	}
	if !foundStringLiteral {
		t.Error("Expected to find 'Done!' string literal")
	}
}

func TestLexComplexArithmetic(t *testing.T) {
	input := `define calculate(a: float, b: float) {
		result: float = a * b + 10.5 / 2.0 - 1;
		return result;
	}`
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Count arithmetic operators
	plusCount := 0
	minusCount := 0
	multiplyCount := 0
	divideCount := 0
	floatCount := 0

	for _, token := range tokens {
		switch token.Type {
		case marigold.Plus:
			plusCount++
		case marigold.Minus:
			minusCount++
		case marigold.Multiply:
			multiplyCount++
		case marigold.Divide:
			divideCount++
		case marigold.FloatLiteral:
			floatCount++
		}
	}

	if plusCount != 1 {
		t.Errorf("Expected 1 plus operator, got %d", plusCount)
	}
	if minusCount != 1 {
		t.Errorf("Expected 1 minus operator, got %d", minusCount)
	}
	if multiplyCount != 1 {
		t.Errorf("Expected 1 multiply operator, got %d", multiplyCount)
	}
	if divideCount != 1 {
		t.Errorf("Expected 1 divide operator, got %d", divideCount)
	}
	if floatCount < 2 {
		t.Errorf("Expected at least 2 float literals, got %d", floatCount)
	}
}

func TestLexNestedBlocks(t *testing.T) {
	input := `define nested() {
		if true {
			while false {
				if something {
					emit "deep";
				}
			}
		}
	}`
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Count braces to ensure proper nesting
	lbraceCount := 0
	rbraceCount := 0

	for _, token := range tokens {
		if token.Type == marigold.LBrace {
			lbraceCount++
		}
		if token.Type == marigold.RBrace {
			rbraceCount++
		}
	}

	if lbraceCount != rbraceCount {
		t.Errorf("Mismatched braces: %d left braces, %d right braces", lbraceCount, rbraceCount)
	}

	if lbraceCount < 4 {
		t.Errorf("Expected at least 4 left braces for nested structure, got %d", lbraceCount)
	}
}

func TestLexCommentsInCode(t *testing.T) {
	input := `define example() {
		// This is a comment
		x: int = 42; // Another comment
		// Final comment
		return x;
	}`
	tokens, err := marigold.Lex(input)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Comments should be ignored, so we shouldn't find any comment tokens
	// But we should find the actual code tokens
	foundDefine := false
	foundInt := false
	foundReturn := false

	for _, token := range tokens {
		if token.Type == marigold.Define {
			foundDefine = true
		}
		if token.Type == marigold.Int {
			foundInt = true
		}
		if token.Type == marigold.Return {
			foundReturn = true
		}
	}

	if !foundDefine {
		t.Error("Expected to find define keyword (comments may have interfered)")
	}
	if !foundInt {
		t.Error("Expected to find int keyword (comments may have interfered)")
	}
	if !foundReturn {
		t.Error("Expected to find return keyword (comments may have interfered)")
	}
}