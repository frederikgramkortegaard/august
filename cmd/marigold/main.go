package main

import (
	"august/marigold"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func parseFile(filename string) {
	fmt.Printf("\n=== Parsing %s ===\n", filename)

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	tokens, err := marigold.Lex(string(content))
	if err != nil {
		fmt.Printf("Lex error: %v\n", err)
		return
	}

	fmt.Printf("Lexed %d tokens\n", len(tokens))

	ast, err := marigold.Parse(tokens)
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}

	// Typecheck the AST
	marigold.Typecheck(ast)

	// Print symbol tables
	marigold.PrintSymbolTables(ast)

	fmt.Printf("Parsed %d functions:\n", len(ast.Functions))
	for _, fn := range ast.Functions {
		fmt.Printf("  - %s(%v) : %s\n", fn.Name, fn.Parameters, fn.ReturnType)
	}

	// Test codegen
	fmt.Printf("\n=== Generating Bytecode ===\n")
	ctx := marigold.CodegenWithContext(ast)
	fmt.Printf("Generated %d instructions\n", len(ctx.Instructions))
	for i, instr := range ctx.Instructions {
		fmt.Printf("  %d: %s %v\n", i, instr.Opcode, instr.Value)
	}

	// Optional: Output JSON for detailed inspection
	for _, arg := range os.Args[1:] {
		if arg == "--json" {
			jsonData, err := json.MarshalIndent(ast, "", "  ")
			if err != nil {
				fmt.Printf("JSON marshal error: %v\n", err)
				return
			}
			fmt.Printf("\nAST JSON:\n%s\n", jsonData)
			break
		}
	}
}

func main() {
	// Check if specific file was provided
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "--") {
		parseFile(os.Args[1])
		return
	}

	// Otherwise parse all files in directory
	contractsDir := "contracts/marigold"

	// Find all .mg files
	files, err := filepath.Glob(filepath.Join(contractsDir, "*.mg"))
	if err != nil {
		fmt.Printf("Error finding .mg files: %v\n", err)
		return
	}

	if len(files) == 0 {
		fmt.Printf("No .mg files found in %s\n", contractsDir)
		return
	}

	fmt.Printf("Found %d Marigold contract files:\n", len(files))
	for _, file := range files {
		parseFile(file)
	}

	fmt.Printf("\nUsage: go run cmd/marigold/main.go [filename.mg] [--json]\n")
}
