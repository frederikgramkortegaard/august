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
		{Opcode: avm.PUSH, Rhs: hex("6")},  // 0x2A = 42 decimal
		{Opcode: avm.PUSH, Rhs: hex("2A")}, // 0x2A = 42 decimal
		{Opcode: avm.PUSH, Rhs: hex("2A")}, // 0x2A = 42 decimal
		{Opcode: avm.POP},
		{Opcode: avm.PUSH, Rhs: hex("DEAD")}, // 0xDEAD
		{Opcode: avm.DUP},
		{Opcode: avm.SWAP, Rhs: hex("5")},
		{Opcode: avm.EMIT},
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
