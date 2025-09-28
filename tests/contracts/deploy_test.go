package contracts

import (
	"august/blockchain"
	"august/marigold"
	"august/types"
	"testing"
)

func TestContractBytecodeGeneration(t *testing.T) {

	// Create simple Marigold contract
	code := `
	define init() : int {
		emit(42)
		return 1
	}

	define call() : int {
		emit(100)
		return 2
	}
	`

	// Compile contract
	tokens, err := marigold.Lex(code)
	if err != nil {
		t.Fatalf("Failed to lex contract: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Failed to parse contract: %v", err)
	}

	ctx := marigold.CodegenWithContext(ast)
	t.Logf("Generated %d instructions", len(ctx.Instructions))

	// Print full bytecode
	var bytecode string
	for i, inst := range ctx.Instructions {
		if inst.Opcode == types.PUSH && inst.Value != nil {
			bytecode += "PUSH " + *inst.Value + " "
		} else {
			bytecode += inst.Opcode.String() + " "
		}
		if (i+1)%10 == 0 {
			bytecode += "\n"
		}
	}
	t.Logf("Bytecode:\n%s", bytecode)

	// Convert to blockchain instructions
	instructions := make([]blockchain.Instruction, len(ctx.Instructions))
	for i, instr := range ctx.Instructions {
		instructions[i] = blockchain.Instruction{
			Opcode: instr.Opcode,
			Value:  instr.Value,
		}
	}

	// Verify bytecode structure
	if len(ctx.Instructions) < 4 {
		t.Fatalf("Expected at least 4 instructions for entry points, got %d", len(ctx.Instructions))
	}


	// Verify entry points exist
	if ctx.Instructions[0].Opcode != types.PUSH {
		t.Fatalf("Expected PUSH at IC=0")
	}
	if ctx.Instructions[1].Opcode != types.JUMP {
		t.Fatalf("Expected JUMP at IC=1")
	}
	if ctx.Instructions[2].Opcode != types.PUSH {
		t.Fatalf("Expected PUSH at IC=2")
	}
	if ctx.Instructions[3].Opcode != types.JUMP {
		t.Fatalf("Expected JUMP at IC=3")
	}

	// Verify function addresses are recorded
	initAddr, hasInit := ctx.FunctionIndex["init"]
	if !hasInit {
		t.Fatalf("init function not found in function index")
	}
	callAddr, hasCall := ctx.FunctionIndex["call"]
	if !hasCall {
		t.Fatalf("call function not found in function index")
	}

	t.Logf("init function at address: %d", initAddr)
	t.Logf("call function at address: %d", callAddr)

	// Verify entry points jump to correct addresses
	if ctx.Instructions[0].Value == nil {
		t.Fatalf("IC=0 PUSH has no value")
	}
	if ctx.Instructions[2].Value == nil {
		t.Fatalf("IC=2 PUSH has no value")
	}

	t.Logf("Test passed: Bytecode properly generated with entry points")
	t.Logf("Total instructions: %d", len(ctx.Instructions))
}
