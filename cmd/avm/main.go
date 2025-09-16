package main

import (
	"august/avm"
	"fmt"
	"math/big"
)

func main() {
	// Helper function for inline hex values
	hex := func(s string) *big.Int {
		n, _ := new(big.Int).SetString(s, 16)
		return n
	}

	// Create a simple calculator: (7 + 3) * 2 > 15 ?
	instructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, hex("7")), // Push 7
		avm.MakeInstructionWithValue(avm.PUSH, hex("3")), // Push 3
		avm.MakeInstruction(avm.ADD),                     // 7 + 3 = 10
		avm.MakeInstructionWithValue(avm.PUSH, hex("2")), // Push 2
		avm.MakeInstruction(avm.MUL),                     // 10 * 2 = 20
		avm.MakeInstructionWithValue(avm.PUSH, hex("F")), // Push 15
		avm.MakeInstruction(avm.GT),                      // 20 > 15 = true (1)
		avm.MakeInstruction(avm.EMIT),                    // Emit result
	}

	// Create runtime with 1000 gas and default config
	config := avm.RuntimeConfig{
		StackSize:  1024, // Allow up to 1024 stack elements
		MemorySize: 1024, // Allow up to 1024 memory slots
	}
	r := avm.NewRuntime(1000, instructions, config)

	// Execute
	gasUsed, err := r.StartExecution()
	if err != nil {
		fmt.Printf("Execution error: %v\n", err)
	} else {
		fmt.Printf("Execution completed. Gas used: %d\n", gasUsed)
	}
}
