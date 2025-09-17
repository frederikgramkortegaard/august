package marigold

import "fmt"

type TypeCheckContext struct {
	ast *Ast
}

func typecheckFunctionDefinition(fd *Function) {

}

func Typecheck(ast *Ast) {

	fmt.Println("Typechecking")
	if ast == nil {
		return
	}
	ctx := &TypeCheckContext{ast: ast}

	for _, funcdef := range ast.Functions {
		typecheckFunctionDefinition(funcdef)
	}

	_ = ctx

}
