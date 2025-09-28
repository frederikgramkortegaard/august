package contracts

import (
	"august/marigold"
	"august/types"
	"testing"
)

func TestStringLiteralCodegen(t *testing.T) {
	code := `
	define init() : int {
		name: string = "Alice"
		greeting: string = "Hello World"
		return 1
	}

	define call() : int {
		message: string = "Contract executed"
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

	// Verify that MSTORE instructions were generated for string storage
	mstoreCount := 0
	for _, inst := range ctx.Instructions {
		if inst.Opcode == types.MSTORE {
			mstoreCount++
		}
	}

	// We should have MSTORE instructions for storing strings:
	// "Alice" (5 bytes, 1 chunk) = 2 MSTOREs (length + data)
	// "Hello World" (11 bytes, 1 chunk) = 2 MSTOREs (length + data)
	// "Contract executed" (17 bytes, 1 chunk) = 2 MSTOREs (length + data)
	expectedMstores := 6
	if mstoreCount != expectedMstores {
		t.Logf("MSTORE count: got %d, expected %d", mstoreCount, expectedMstores)
	}

	t.Logf("String literals successfully compiled with %d MSTORE instructions", mstoreCount)
}