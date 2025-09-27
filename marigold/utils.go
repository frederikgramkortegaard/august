package marigold

import "fmt"

func PrintSymbolTables(ast *Ast) {
	fmt.Println("\n=== Symbol Tables ===")

	// Print global scope
	if ast.Scope != nil {
		fmt.Println("Global Scope:")
		printScope(ast.Scope, "  ")
	}

	// Print function scopes
	for _, fn := range ast.Functions {
		fmt.Printf("\nFunction '%s' Scope:\n", fn.Name)
		if fn.Block != nil && fn.Block.Scope != nil {
			printScope(fn.Block.Scope, "  ")
		}
		printBlockScopes(fn.Block, "  ")
	}
}

func printScope(scope *Scope, indent string) {
	if scope == nil {
		fmt.Printf("%s<nil scope>\n", indent)
		return
	}

	if len(scope.Variables) == 0 {
		fmt.Printf("%s(empty)\n", indent)
	} else {
		for name, variable := range scope.Variables {
			fmt.Printf("%s%s: %s (type: %s)\n", indent, name, variable.Value, variable.Type)
		}
	}
}

func printBlockScopes(block *Block, indent string) {
	if block == nil {
		return
	}

	for _, stmt := range block.Statements {
		if stmt.Block != nil {
			fmt.Printf("%sNested Block Scope:\n", indent)
			printScope(stmt.Block.Scope, indent+"  ")
			printBlockScopes(stmt.Block, indent+"  ")
		}
		if stmt.ElseBlock != nil {
			fmt.Printf("%sElse Block Scope:\n", indent)
			printScope(stmt.ElseBlock.Scope, indent+"  ")
			printBlockScopes(stmt.ElseBlock, indent+"  ")
		}
	}
}
