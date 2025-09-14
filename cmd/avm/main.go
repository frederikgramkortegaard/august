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

	// Create a list of instructions: push, pop, push, dup, emit
	instructions := []avm.Instruction{
		avm.MakeInstructionWithValue(avm.PUSH, hex("6")),     // 0x06 = 6 decimal
		avm.MakeInstructionWithValue(avm.PUSH, hex("2A")),    // 0x2A = 42 decimal
		avm.MakeInstructionWithValue(avm.PUSH, hex("2A")),    // 0x2A = 42 decimal
		avm.MakeInstruction(avm.POP),
		avm.MakeInstructionWithValue(avm.PUSH, hex("DEAD")),  // 0xDEAD
		avm.MakeInstruction(avm.DUP),
		avm.MakeInstructionWithParam(avm.SWAP, 3),
		avm.MakeInstruction(avm.EMIT),
	}

	// Create runtime with 1000 gas
	r := avm.NewRuntime(1000)

	// Execute
	gasUsed, err := r.StartExecution(instructions)
	if err != nil {
		fmt.Printf("Execution error: %v\n", err)
	} else {
		fmt.Printf("Execution completed. Gas used: %d\n", gasUsed)
	}
}
