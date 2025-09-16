package main

import (
	"august/avm"
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	var runtimeCode string

	if len(os.Args) > 1 {
		// Load from file if provided
		filename := os.Args[1]
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", filename, err)
			os.Exit(1)
		}
		runtimeCode = string(data)
		fmt.Printf("Loaded bytecode from %s\n", filename)
	} else {
		fmt.Printf("No contract file given to parse")
		os.Exit(1)
	}

	// Parse the bytecode
	instructions, err := avm.ParseInstructionsFromString(runtimeCode)
	if err != nil {
		fmt.Printf("Parse Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully parsed %d instructions:\n", len(instructions))
	for i, inst := range instructions {
		fmt.Printf("[%d] %+v\n", i, inst)
	}

	// Test round-trip conversion
	fmt.Println("\nRound-trip test:")
	exported := avm.ExportInstructionsAsString(instructions)
	fmt.Printf("Exported: %s\n", exported)

	// Verify round-trip works
	reparsed, err := avm.ParseInstructionsFromString(exported)
	if err != nil {
		fmt.Printf("Round-trip Error: %v\n", err)
		os.Exit(1)
	}

	if len(reparsed) == len(instructions) {
		fmt.Printf("Round-trip successful: %d instructions\n", len(reparsed))
	} else {
		fmt.Printf("Round-trip failed: %d != %d\n", len(reparsed), len(instructions))
		os.Exit(1)
	}
}
