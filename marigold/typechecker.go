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

func typecheckExpression(ctx *TypeCheckContext, expr *Expression) Type {
	if expr == nil {
		panic("expr is nil")
	}

	switch expr.Type {
	case UnaryExpr:
		rhsType := typecheckExpression(ctx, expr.Rhs)

		if !isValidUnaryOperatorUse(expr.Operator, rhsType) {
			ctx.logFatal(fmt.Sprintf("Invalid operator '%s' for type '%s'", expr.Operator, rhsType.String()))
		}

		return rhsType

	case LiteralExpr:
		return TypeFromTokenType(expr.ValueType)

	case IdentifierExpr:
		if expr.Value == nil {
			ctx.logFatalWithToken("Identifier expression has nil value", expr.Token)
		}

		// Lookup this identifier in the current scope
		varName := expr.Value.(string)
		variable := ctx.currentScope.findVariable(varName)
		if variable != nil {
			return variable.Type
		}

		// If not found as variable, check if it's a function
		if _, exists := ctx.ast.Functions[varName]; exists {
			return TypeFromTokenType(TFunction)
		}

		ctx.logFatalWithToken(fmt.Sprintf("Undefined variable or function '%s'", expr.Value), expr.Token)

	case BinaryExpr:
		lhs := typecheckExpression(ctx, expr.Lhs)
		rhs := typecheckExpression(ctx, expr.Rhs)
		if !isValidBinaryOperatorUse(expr.Operator, lhs, rhs) {
			ctx.logFatalWithToken(fmt.Sprintf("Invalid binary operation: cannot use operator '%s' with types '%s' and '%s'", expr.Operator, lhs.String(), rhs.String()), expr.Token)
		}

		// Comparison operators return Bool
		if expr.Operator == LessThan || expr.Operator == LessEqual ||
			expr.Operator == GreaterThan || expr.Operator == GreaterEqual ||
			expr.Operator == Equal || expr.Operator == NotEqual {
			return BoolType
		}

		// Logical operators return Bool
		if expr.Operator == LogicalAnd || expr.Operator == LogicalOr {
			return BoolType
		}

		// Handle return types for arithmetic operations
		if expr.Operator == Plus {
			if lhs.Equals(StringType) && rhs.Equals(StringType) {
				return StringType // String concatenation
			}
			// Numeric addition: if either is float, result is float
			if lhs.Equals(FloatType) || rhs.Equals(FloatType) {
				return FloatType
			}
			return IntType // Both are int
		}

		if expr.Operator == Divide {
			// Division always returns float (even int/int)
			return FloatType
		}

		if expr.Operator == Minus || expr.Operator == Multiply {
			// If either operand is float, result is float
			if lhs.Equals(FloatType) || rhs.Equals(FloatType) {
				return FloatType
			}
			return IntType // Both are int
		}

		// Shouldn't reach here
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

			// Check that argument is an array or string
			argType := typecheckExpression(ctx, expr.Args[0])

			if argType.Equals(StringType) {
				return IntType
			}

			if _, ok := argType.(*ArrayType); ok {
				return IntType
			}

			if _, ok := argType.(*MapType); ok {
				return IntType
			}

			ctx.logFatalWithToken(fmt.Sprintf("len() can only be used on arrays, maps, or strings, got '%s'", argType.String()), expr.Token)
			return IntType
		}

		if funcName == "emit" {
			if len(expr.Args) != 1 {
				ctx.logFatalWithToken(fmt.Sprintf("emit() expects 1 argument, got %d", len(expr.Args)), expr.Token)
			}

			// emit() can only take simple types that can be printed
			argType := typecheckExpression(ctx, expr.Args[0])

			if !argType.Equals(IntType) && !argType.Equals(FloatType) &&
			   !argType.Equals(StringType) && !argType.Equals(BoolType) {
				ctx.logFatalWithToken(fmt.Sprintf("emit() can only print simple types (int, float, string, bool), got '%s'", argType.String()), expr.Token)
			}

			// emit() doesn't return a value (void function)
			return AnyType  // Use AnyType as placeholder for void
		}

		if funcName == "stop" {
			if len(expr.Args) != 0 {
				ctx.logFatalWithToken(fmt.Sprintf("stop() expects no arguments, got %d", len(expr.Args)), expr.Token)
			}

			// stop() doesn't return a value (exits program)
			return AnyType  // Use AnyType as placeholder for void
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
			if !argType.IsAssignableTo(paramType) {
				ctx.logFatal(fmt.Sprintf("Function '%s' parameter %d expects type '%s' but got '%s'", expr.Value.(string), idx+1, paramType.String(), argType.String()))
			}
		}

		return fd.ReturnType

	case ArrayLiteral:
		// Array literal [1, 2, 3]
		if len(expr.Args) == 0 {
			// Empty array [] - type must be determined from context
			return nil
		}

		// Check all elements have same type
		firstType := typecheckExpression(ctx, expr.Args[0])
		for i, element := range expr.Args[1:] {
			elementType := typecheckExpression(ctx, element)
			if !elementType.Equals(firstType) {
				ctx.logFatalWithToken(fmt.Sprintf("Array element %d has type '%s' but expected '%s'", i+2, elementType.String(), firstType.String()), element.Token)
			}
		}

		return NewArrayType(-1, firstType) // Inferred size array

	case MapLiteral:
		// Map literal {} - currently only empty maps supported
		return nil // Type must be determined from context

	case IndexExpr:
		// Array, map, or string indexing
		lhsType := typecheckExpression(ctx, expr.Lhs)
		indexType := typecheckExpression(ctx, expr.Rhs)

		// Handle string indexing
		if lhsType.Equals(StringType) {
			// String indexing requires int
			if !indexType.IsAssignableTo(IntType) {
				ctx.logFatalWithToken(fmt.Sprintf("String index must be int, got '%s'", indexType.String()), expr.Token)
			}
			return StringType // str[i] returns a single-character string
		}

		// Handle array indexing
		if arrType, ok := lhsType.(*ArrayType); ok {
			// Array indexing requires int
			if !indexType.IsAssignableTo(IntType) {
				ctx.logFatalWithToken(fmt.Sprintf("Array index must be int, got '%s'", indexType.String()), expr.Token)
			}
			return arrType.ElementType
		}

		// Handle map indexing
		if mapType, ok := lhsType.(*MapType); ok {
			// Map indexing requires string key (for now)
			if !indexType.Equals(StringType) {
				ctx.logFatalWithToken(fmt.Sprintf("Map index must be string, got '%s'", indexType.String()), expr.Token)
			}
			return mapType.ValueType
		}

		ctx.logFatalWithToken(fmt.Sprintf("Cannot index expression of type '%s'", lhsType.String()), expr.Token)
		return nil
	}

	return nil
}

func isValidUnaryOperatorUse(op TokenType, operandType Type) bool {
	switch op {
	case Minus:
		// Unary minus only valid for numeric types
		return operandType.IsNumeric()
	default:
		return false
	}
}

func isValidBinaryOperatorUse(op TokenType, lhs, rhs Type) bool {
	switch op {
	case Plus:
		// Plus works on numeric types AND strings (concatenation)
		if lhs.Equals(StringType) && rhs.Equals(StringType) {
			return true // String concatenation
		}
		// Numeric addition: allow int/float mixing
		return lhs.IsNumeric() && rhs.IsNumeric()
	case Minus, Multiply, Divide:
		// Arithmetic operators work on numeric types, allow int/float mixing
		return lhs.IsNumeric() && rhs.IsNumeric()
	case LessThan, LessEqual, GreaterThan, GreaterEqual:
		// Comparison operators work on numeric types and strings
		if lhs.Equals(StringType) && rhs.Equals(StringType) {
			return true // String comparison
		}
		// Numeric comparison: allow int/float mixing
		return lhs.IsNumeric() && rhs.IsNumeric()
	case Equal, NotEqual:
		// Equality operators work on compatible types
		return lhs.IsAssignableTo(rhs) || rhs.IsAssignableTo(lhs)
	case LogicalAnd, LogicalOr:
		// Logical operators only work on Bool
		return lhs.Equals(BoolType) && rhs.Equals(BoolType)
	default:
		return false
	}
}

func typecheckBlock(ctx *TypeCheckContext, block *Block) {
	for _, stmt := range block.Statements {
		switch stmt.Type {
		case AssignmentStmt:
			// Check if this is a variable declaration (has VarType) or reassignment
			if stmt.VarType != nil {
				// Variable declaration
				if stmt.Lhs == nil || stmt.Lhs.Value == nil {
					ctx.logFatal("Assignment statement has invalid left-hand side")
				}
				varName := stmt.Lhs.Value.(string)

				// Check if variable already exists in current scope
				if _, exists := ctx.currentScope.Variables[varName]; exists {
					ctx.logFatal(fmt.Sprintf("Variable '%s' is already declared in this scope", varName))
				}

				// Type check RHS if present
				if stmt.Rhs != nil {
					rhsType := typecheckExpression(ctx, stmt.Rhs)

					// For array/map literals, we need special handling
					if rhsType == nil {
						// Empty array or map literal, type is from declaration
						rhsType = stmt.VarType
					}

					// Special case: array declaration
					if declArray, ok := stmt.VarType.(*ArrayType); ok {
						if rhsArray, ok := rhsType.(*ArrayType); ok {
							// Check array literal size matches declaration
							if stmt.Rhs.Type == ArrayLiteral {
								literalSize := len(stmt.Rhs.Args)
								if declArray.Size == -1 {
									// Inferred size: update from literal
									declArray.Size = literalSize
									rhsArray.Size = literalSize
								} else if rhsArray.Size == -1 {
									// Literal has inferred size, check against declared size
									if declArray.Size != literalSize {
										ctx.logFatal(fmt.Sprintf("Array literal has %d elements but variable '%s' declared as [%d]%s",
											literalSize, varName, declArray.Size, declArray.ElementType.String()))
									}
									rhsArray.Size = literalSize
								}
							}
						}
					}

					if !rhsType.IsAssignableTo(stmt.VarType) {
						ctx.logFatal(fmt.Sprintf("Cannot assign '%s' to variable '%s' of type '%s'", rhsType.String(), varName, stmt.VarType.String()))
					}
				}

				// Add variable to current scope
				ctx.currentScope.Variables[varName] = &Variable{
					Name:  varName,
					Value: "",
					Type:  stmt.VarType,
				}
			} else {
				// Variable reassignment or indexed assignment
				if stmt.Lhs == nil {
					ctx.logFatal("Assignment statement has invalid left-hand side")
				}

				if stmt.Lhs.Type == IdentifierExpr {
					// Simple variable reassignment
					if stmt.Lhs.Value == nil {
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
					if !rhsType.IsAssignableTo(variable.Type) {
						ctx.logFatal(fmt.Sprintf("Cannot assign '%s' to variable '%s' of type '%s'", rhsType.String(), varName, variable.Type.String()))
					}
				} else if stmt.Lhs.Type == IndexExpr {
					// Indexed assignment: map["key"] = value or arr[index] = value
					// Type check the LHS (this validates the indexing)
					lhsType := typecheckExpression(ctx, stmt.Lhs)

					// Type check the RHS expression
					rhsType := typecheckExpression(ctx, stmt.Rhs)

					// Check if RHS type matches the expected element/value type
					if !rhsType.IsAssignableTo(lhsType) {
						ctx.logFatal(fmt.Sprintf("Cannot assign '%s' to indexed location of type '%s'", rhsType.String(), lhsType.String()))
					}
				} else {
					ctx.logFatal("Invalid assignment target")
				}
			}
		case ReturnStmt:
			actualType := typecheckExpression(ctx, stmt.Rhs)
			expectedType := ctx.currentFunction.ReturnType

			if actualType == nil {
				// Empty literal, check if return type allows it
				if _, ok := expectedType.(*ArrayType); ok {
					actualType = expectedType // Allow empty array literal
				} else if _, ok := expectedType.(*MapType); ok {
					actualType = expectedType // Allow empty map literal
				}
			}

			if !actualType.IsAssignableTo(expectedType) {
				panic(fmt.Sprintf("Type mismatch in return statement: expected %s but got %s in function %s", expectedType.String(), actualType.String(), ctx.currentFunction.Name))
			}

		case IfStmt:
			// Type check the condition - should be Bool
			conditionType := typecheckExpression(ctx, stmt.Conditional)
			if !conditionType.Equals(BoolType) {
				ctx.logFatalWithToken(fmt.Sprintf("If condition must be Bool, got %s", conditionType.String()), stmt.Token)
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
			if !conditionType.Equals(BoolType) {
				ctx.logFatalWithToken(fmt.Sprintf("While condition must be Bool, got %s", conditionType.String()), stmt.Token)
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

	// Check for required main function
	mainFunc, hasMain := ast.Functions["main"]
	if !hasMain {
		ctx.logFatal("Program must have a main() function")
	}

	// Validate main function signature: main() : int
	if len(mainFunc.Parameters) != 0 {
		ctx.logFatal("main() function must take no parameters")
	}

	if !mainFunc.ReturnType.Equals(IntType) {
		ctx.logFatal("main() function must return int")
	}

	for _, funcdef := range ast.Functions {
		typecheckFunctionDefinition(ctx, funcdef)
	}
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