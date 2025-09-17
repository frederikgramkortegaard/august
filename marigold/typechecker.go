package marigold

import (
	"fmt"
	"log"
)

type TypeCheckContext struct {
	ast             *Ast
	currentFunction *Function
	currentScope    *Scope
}

func (ctx *TypeCheckContext) logFatal(message string) {
	if ctx.currentFunction != nil {
		log.Panicf("Type error in function '%s': %s", ctx.currentFunction.Name, message)
	} else {
		log.Panicf("Type error: %s", message)
	}
}

func (ctx *TypeCheckContext) logFatalWithToken(message string, token *Token) {
	if token != nil {
		if ctx.currentFunction != nil {
			log.Panicf("Type error in function '%s': %s at %s:%d:%d", ctx.currentFunction.Name, message, token.Filename, token.Row+1, token.Col+1)
		} else {
			log.Panicf("Type error: %s at %s:%d:%d", message, token.Filename, token.Row+1, token.Col+1)
		}
	} else {
		ctx.logFatal(message)
	}
}

func typecheckExpression(ctx *TypeCheckContext, expr *Expression) TokenType {

	if expr == nil {
		panic("expr is nil")
	}

	switch expr.Type {
	case UnaryExpr:

		rhsType := typecheckExpression(ctx, expr.Rhs)

		if !isValidUnaryOperatorUse(expr.Operator, rhsType) {
			ctx.logFatal(fmt.Sprintf("Invalid operator '%s' for type '%s'", expr.Operator, rhsType))
		}

		return rhsType

	case LiteralExpr:
		return expr.ValueType

	case IdentifierExpr:
		if expr.Value == nil {
			ctx.logFatalWithToken("Identifier expression has nil value", expr.Token)
		}

		// Lookup this identifier in the current scope
		variable := ctx.currentScope.findVariable(expr.Value.(string))
		if variable != nil {
			if variable.ArrayType != nil {
				// This is an array variable - return special array type marker
				// The specific element type will be handled in IndexExpr
				return "ARRAY" // Temporary marker
			}
			return variable.Type
		}

		// If not found as variable, check if it's a function
		funcName := expr.Value.(string)
		if _, exists := ctx.ast.Functions[funcName]; exists {
			return TFunction
		}

		ctx.logFatalWithToken(fmt.Sprintf("Undefined variable or function '%s'", expr.Value), expr.Token)

	case BinaryExpr:
		lhs := typecheckExpression(ctx, expr.Lhs)
		rhs := typecheckExpression(ctx, expr.Rhs)
		if !isValidBinaryOperatorUse(expr.Operator, lhs, rhs) {
			ctx.logFatalWithToken(fmt.Sprintf("Invalid binary operation: cannot use operator '%s' with types '%s' and '%s'", expr.Operator, lhs, rhs), expr.Token)
		}

		// Comparison operators return Bool
		if expr.Operator == LessThan || expr.Operator == LessEqual ||
		   expr.Operator == GreaterThan || expr.Operator == GreaterEqual ||
		   expr.Operator == Equal || expr.Operator == NotEqual {
			return Bool
		}

		// Logical operators return Bool
		if expr.Operator == LogicalAnd || expr.Operator == LogicalOr {
			return Bool
		}

		// Arithmetic operators return the left operand type
		return lhs

	case CallExpr:
		if expr.Value == nil {
			ctx.logFatalWithToken("Function call expression has nil value", expr.Token)
		}

		funcName := expr.Value.(string)

		// Handle built-in functions
		if funcName == "len" {
			if len(expr.Args) != 1 {
				ctx.logFatalWithToken(fmt.Sprintf("len() expects 1 argument, got %d", len(expr.Args)), expr.Token)
			}

			// Check that argument is an array
			argExpr := expr.Args[0]
			if argExpr.Type == IdentifierExpr {
				varName := argExpr.Value.(string)
				variable := ctx.currentScope.findVariable(varName)
				if variable == nil {
					ctx.logFatalWithToken(fmt.Sprintf("Undefined variable '%s'", varName), argExpr.Token)
				}
				if variable.ArrayType == nil {
					ctx.logFatalWithToken(fmt.Sprintf("len() can only be used on arrays, got variable of type '%s'", variable.Type), argExpr.Token)
				}
			} else {
				ctx.logFatalWithToken("len() argument must be an array variable", argExpr.Token)
			}

			return Int // len() returns int
		}

		fd, ok := ctx.ast.Functions[funcName]
		if !ok {
			ctx.logFatalWithToken(fmt.Sprintf("Function '%s' does not exist", funcName), expr.Token)
		}

		if len(expr.Args) != len(fd.ParameterTypes) {
			ctx.logFatalWithToken(fmt.Sprintf("Function '%s' expects %d arguments but got %d", expr.Value.(string), len(fd.ParameterTypes), len(expr.Args)), expr.Token)
		}
		for idx, arg := range expr.Args {
			argType := typecheckExpression(ctx, arg)
			paramType := fd.ParameterTypes[idx]
			if argType != paramType {
				ctx.logFatal(fmt.Sprintf("Function '%s' parameter %d expects type '%s' but got '%s'", expr.Value.(string), idx+1, paramType, argType))
			}
		}

		return fd.ReturnType

	case ArrayLiteral:
		// Array literal [1, 2, 3] - type will be inferred from context
		if len(expr.Args) == 0 {
			// Empty array [] - type must be determined from context
			return ""
		}

		// Check all elements have same type
		firstType := typecheckExpression(ctx, expr.Args[0])
		for i, element := range expr.Args[1:] {
			elementType := typecheckExpression(ctx, element)
			if elementType != firstType {
				ctx.logFatalWithToken(fmt.Sprintf("Array element %d has type '%s' but expected '%s'", i+2, elementType, firstType), element.Token)
			}
		}

		return firstType // Return element type, array type will be handled in assignment

	case IndexExpr:
		// Array indexing arr[i]
		_ = typecheckExpression(ctx, expr.Lhs) // Validate the array expression
		indexType := typecheckExpression(ctx, expr.Rhs)

		// Index must be int
		if indexType != Int {
			ctx.logFatalWithToken(fmt.Sprintf("Array index must be int, got '%s'", indexType), expr.Token)
		}

		// Get the actual array variable to find element type
		if expr.Lhs.Type == IdentifierExpr {
			varName := expr.Lhs.Value.(string)
			variable := ctx.currentScope.findVariable(varName)
			if variable != nil && variable.ArrayType != nil {
				return variable.ArrayType.ElementType
			}
		}

		ctx.logFatalWithToken(fmt.Sprintf("Cannot index non-array expression"), expr.Token)
		return ""

	}

	return ""
}

func isValidUnaryOperatorUse(op TokenType, operandType TokenType) bool {
	switch op {
	case Minus:
		// Unary minus only valid for numeric types
		return operandType == Int || operandType == Float
	default:
		return false
	}
}

func isValidBinaryOperatorUse(op, lhs, rhs TokenType) bool {
	// Types must match for most operations
	if lhs != rhs {
		return false
	}

	switch op {
	case Plus, Minus, Multiply, Divide:
		// Arithmetic operators only work on numeric types
		return lhs == Int || lhs == Float
	case LessThan, LessEqual, GreaterThan, GreaterEqual:
		// Comparison operators work on numeric types and strings
		return lhs == Int || lhs == Float || lhs == String
	case Equal, NotEqual:
		// Equality operators work on all types
		return true
	case LogicalAnd, LogicalOr:
		// Logical operators only work on Bool
		return lhs == Bool
	default:
		return false
	}
}
func typecheckBlock(ctx *TypeCheckContext, block *Block) {

	for _, stmt := range block.Statements {

		switch stmt.Type {
		case AssignmentStmt:
			// Check if this is a variable declaration (has VarType/VarArrayType) or reassignment
			if stmt.VarType != "" || stmt.VarArrayType != nil {
				// Variable declaration: x: int = 5 or arr: [5]int = [1,2,3,4,5]
				if stmt.Lhs == nil || stmt.Lhs.Value == nil {
					ctx.logFatal("Assignment statement has invalid left-hand side")
				}
				varName := stmt.Lhs.Value.(string)

				// Check if variable already exists in current scope
				if _, exists := ctx.currentScope.Variables[varName]; exists {
					ctx.logFatal(fmt.Sprintf("Variable '%s' is already declared in this scope", varName))
				}

				if stmt.VarArrayType != nil {
					// Array variable declaration
					if stmt.Rhs != nil {
						// Has initializer: arr: [5]int = [1,2,3,4,5]
						rhsType := typecheckExpression(ctx, stmt.Rhs)

						// Check element type matches
						if rhsType != stmt.VarArrayType.ElementType {
							ctx.logFatal(fmt.Sprintf("Cannot assign array of '%s' to variable '%s' of type '[%d]%s'", rhsType, varName, stmt.VarArrayType.Size, stmt.VarArrayType.ElementType))
						}

						// If size is inferred ([]int), set it from array literal
						if stmt.VarArrayType.Size == -1 && stmt.Rhs.Type == ArrayLiteral {
							stmt.VarArrayType.Size = len(stmt.Rhs.Args)
						}

						// Check array literal size matches declared size
						if stmt.Rhs.Type == ArrayLiteral && len(stmt.Rhs.Args) != stmt.VarArrayType.Size {
							ctx.logFatal(fmt.Sprintf("Array literal has %d elements but variable '%s' declared as [%d]%s", len(stmt.Rhs.Args), varName, stmt.VarArrayType.Size, stmt.VarArrayType.ElementType))
						}
					}

					// Add array variable to current scope
					ctx.currentScope.Variables[varName] = &Variable{
						Name:      varName,
						Value:     "",
						Type:      "", // Empty for arrays
						ArrayType: stmt.VarArrayType,
					}
				} else {
					// Regular variable declaration
					rhsType := typecheckExpression(ctx, stmt.Rhs)

					// Check if RHS type matches declared type
					if rhsType != stmt.VarType {
						ctx.logFatal(fmt.Sprintf("Cannot assign '%s' to variable '%s' of type '%s'", rhsType, varName, stmt.VarType))
					}

					// Add variable to current scope
					ctx.currentScope.Variables[varName] = &Variable{
						Name:  varName,
						Value: "",
						Type:  stmt.VarType,
					}
				}
			} else {
				// Variable reassignment: x = 10
				if stmt.Lhs == nil || stmt.Lhs.Value == nil {
					ctx.logFatal("Assignment statement has invalid left-hand side")
				}
				varName := stmt.Lhs.Value.(string)

				// Look up existing variable
				variable := ctx.currentScope.findVariable(varName)
				if variable == nil {
					ctx.logFatal(fmt.Sprintf("Undefined variable '%s'", varName))
				}

				// Type check the RHS expression
				rhsType := typecheckExpression(ctx, stmt.Rhs)

				// Check if RHS type matches variable's type
				if rhsType != variable.Type {
					ctx.logFatal(fmt.Sprintf("Cannot assign '%s' to variable '%s' of type '%s'", rhsType, varName, variable.Type))
				}
			}
		case ReturnStmt:
			actualType := typecheckExpression(ctx, stmt.Rhs)
			expectedType := ctx.currentFunction.ReturnType
			if actualType != expectedType {
				panic(fmt.Sprintf("Type mismatch in return statement: expected %s but got %s in function %s", expectedType, actualType, ctx.currentFunction.Name))
			}

		case IfStmt:
			// Type check the condition - should be Bool
			conditionType := typecheckExpression(ctx, stmt.Conditional)
			if conditionType != Bool {
				ctx.logFatalWithToken(fmt.Sprintf("If condition must be Bool, got %s", conditionType), stmt.Token)
			}
			// Type check the if block with its scope
			if stmt.Block != nil {
				previousScope := ctx.currentScope
				ctx.currentScope = stmt.Block.Scope
				typecheckBlock(ctx, stmt.Block)
				ctx.currentScope = previousScope
			}
			// Type check the else block if it exists
			if stmt.ElseBlock != nil {
				previousScope := ctx.currentScope
				ctx.currentScope = stmt.ElseBlock.Scope
				typecheckBlock(ctx, stmt.ElseBlock)
				ctx.currentScope = previousScope
			}

		case WhileStmt:
			// Type check the condition - should be Bool
			conditionType := typecheckExpression(ctx, stmt.Conditional)
			if conditionType != Bool {
				ctx.logFatalWithToken(fmt.Sprintf("While condition must be Bool, got %s", conditionType), stmt.Token)
			}
			// Type check the while block with its scope
			if stmt.Block != nil {
				previousScope := ctx.currentScope
				ctx.currentScope = stmt.Block.Scope
				typecheckBlock(ctx, stmt.Block)
				ctx.currentScope = previousScope
			}

		case ExpressionStmt:
			// Type check the expression (could be function call, etc.)
			typecheckExpression(ctx, stmt.Rhs)
		case EmitStmt:
			// Type check the emitted expression
			typecheckExpression(ctx, stmt.Rhs)
		default:
			ctx.logFatal(fmt.Sprintf("Unknown statement type '%s'", stmt.Type))
		}
	}
}

func typecheckFunctionDefinition(ctx *TypeCheckContext, fd *Function) {

	ctx.currentFunction = fd
	ctx.currentScope = fd.Block.Scope
	scope := fd.Block.Scope

	// Inject Parameters into Symbol Table in Scope
	if len(fd.Parameters) != len(fd.ParameterTypes) {
		ctx.logFatal(fmt.Sprintf("Function '%s' has mismatched parameter count: %d names vs %d types", fd.Name, len(fd.Parameters), len(fd.ParameterTypes)))
	}

	for idx, paramName := range fd.Parameters {

		if _, ok := scope.Variables[paramName]; ok {
			ctx.logFatal(fmt.Sprintf("Parameter '%s' is already defined in function '%s'", paramName, fd.Name))
		}

		paramType := fd.ParameterTypes[idx]

		scope.Variables[paramName] = &Variable{
			Name:  paramName,
			Value: "",
			Type:  paramType,
		}
	}

	// Rest of the function
	typecheckBlock(ctx, fd.Block)

}

func Typecheck(ast *Ast) {

	fmt.Println("Typechecking")
	if ast == nil {
		return
	}
	ctx := &TypeCheckContext{
		ast:          ast,
		currentScope: ast.Scope,
	}

	for _, funcdef := range ast.Functions {
		typecheckFunctionDefinition(ctx, funcdef)
	}

	_ = ctx

}

func (s *Scope) findVariable(name string) *Variable {
	if val, ok := s.Variables[name]; ok {
		return val
	} else if s.Parent == nil {
		return nil
	} else {
		return s.Parent.findVariable(name)
	}
}
