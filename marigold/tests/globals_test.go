package tests

import (
	"august/marigold"
	"testing"
)

func TestGlobalVariables(t *testing.T) {
	// Test that global variables are available
	input := `define test() : string {
		memory[0] = "test"
		return memory[0]
	}`

	tokens, err := marigold.Lex(input)
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Check that global scope has memory and persistent
	if ast.Scope == nil {
		t.Fatalf("AST global scope is nil")
	}

	if _, ok := ast.Scope.Variables["memory"]; !ok {
		t.Fatalf("memory variable not found in global scope")
	}

	if _, ok := ast.Scope.Variables["persistent"]; !ok {
		t.Fatalf("persistent variable not found in global scope")
	}

	// Test typechecking
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Typecheck failed: %v", r)
		}
	}()

	marigold.Typecheck(ast)
}

func TestPersistentVariable(t *testing.T) {
	// Test persistent map usage
	input := `define test() : string {
		persistent["key"] = "value"
		return persistent["key"]
	}`

	tokens, err := marigold.Lex(input)
	if err != nil {
		t.Fatalf("Lex error: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Test typechecking
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Typecheck failed: %v", r)
		}
	}()

	marigold.Typecheck(ast)
}
