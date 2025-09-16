package main

import (
	"august/avm"
	"fmt"
)

func main() {
	// Create a simple calculator: (7 + 3) * 2 > 15 ?
	instructions := []avm.Instruction{
		avm.MakePushInstruction("0x7"), // Push 7
		avm.MakePushInstruction("0x3"), // Push 3
		avm.MakeInstruction(avm.ADD),   // 7 + 3 = 10
		avm.MakePushInstruction("0x2"), // Push 2
		avm.MakeInstruction(avm.MUL),   // 10 * 2 = 20
		avm.MakePushInstruction("0xF"), // Push 15
		avm.MakeInstruction(avm.GT),    // 20 > 15 = true (1)
		avm.MakeInstruction(avm.EMIT),  // Emit result
	}

	// Create runtime with 1000 gas and default config
	r := avm.NewRuntime(1000, instructions)

	// Execute
	gasUsed, err := r.StartExecution()
	if err != nil {
		fmt.Printf("Execution error: %v\n", err)
	} else {
		fmt.Printf("Execution completed. Gas used: %d\n", gasUsed)
	}
}
