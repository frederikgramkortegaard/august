package contracts

import (
	"august/marigold"
	"august/types"
	"testing"
)

func TestStringOperations(t *testing.T) {
	code := `
	define init() : int {
		name: string = "Alice"
		emit(name)

		greeting: string = "Hello"
		world: string = " World"
		message: string = greeting + world
		emit(message)

		length: int = len(message)
		emit(length)

		literal: string = "A" + "B"
		emit(literal)

		return 1
	}

	define call() : int {
		return 1
	}
	`

	tokens, err := marigold.Lex(code)
	if err != nil {
		t.Fatalf("Failed to lex: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	marigold.Typecheck(ast)

	ctx := marigold.CodegenWithContext(ast)
	if ctx == nil {
		t.Fatal("Codegen returned nil")
	}

	t.Logf("Generated %d instructions", len(ctx.Instructions))

	mstoreCount := 0
	strconcatCount := 0
	strlenCount := 0
	emitstrCount := 0

	for _, inst := range ctx.Instructions {
		switch inst.Opcode {
		case types.MSTORE:
			mstoreCount++
		case types.STRCONCAT:
			strconcatCount++
		case types.STRLEN:
			strlenCount++
		case types.EMITSTR:
			emitstrCount++
		}
	}

	t.Logf("MSTORE instructions: %d", mstoreCount)
	t.Logf("STRCONCAT instructions: %d", strconcatCount)
	t.Logf("STRLEN instructions: %d", strlenCount)
	t.Logf("EMITSTR instructions: %d", emitstrCount)

	if strconcatCount != 2 {
		t.Errorf("Expected 2 STRCONCAT (variables and literals with +), got %d", strconcatCount)
	}
	if strlenCount != 1 {
		t.Errorf("Expected 1 STRLEN, got %d", strlenCount)
	}
	if emitstrCount != 3 {
		t.Errorf("Expected 3 EMITSTR (via emit() auto-detection), got %d", emitstrCount)
	}

	t.Log("String operations compiled successfully")
}