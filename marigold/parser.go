package marigold

import (
	"fmt"
	"log"
)

type ParserContext struct {
	tokens       []*Token
	ast          *Ast
	cursor       int
	currentScope *Scope
}

func (ctx *ParserContext) peek() *Token {
	if ctx.cursor < len(ctx.tokens) {
		return ctx.tokens[ctx.cursor]
	}
	return nil
}

func (ctx *ParserContext) consume() *Token {
	if ctx.cursor < len(ctx.tokens) {
		ctx.cursor++
		return ctx.tokens[ctx.cursor-1]
	}
	return nil
}

func (ctx *ParserContext) consumeAssert(toktype TokenType) *Token {
	if ctx.cursor < len(ctx.tokens) {
		token := ctx.tokens[ctx.cursor]
		if token.Type == toktype {
			ctx.cursor++
		} else {
			ctx.logError(fmt.Sprintf("Expected '%s' but got '%s'", toktype, token.Type), token)
			panic("Parse error")
		}
	} else {
		ctx.logError("End of file", nil)
		panic("Parse error")
	}

	return ctx.tokens[ctx.cursor-1]

}

func (ctx *ParserContext) assert(toktype TokenType) {
	if ctx.cursor < len(ctx.tokens) {
		token := ctx.tokens[ctx.cursor]
		if token.Type != toktype {
			ctx.logError(fmt.Sprintf("Expected '%s' but got '%s'", toktype, token.Type), token)
			panic("Parse error")
		}
	} else {
		ctx.logError("End of file", nil)
		panic("Parse error")
	}
}
func (ctx *ParserContext) consumeIf(toktype TokenType) *Token {
	if ctx.cursor < len(ctx.tokens) {
		if ctx.tokens[ctx.cursor].Type == toktype {
			ctx.cursor++
			return ctx.tokens[ctx.cursor-1]
		}
	}
	return nil
}
func (ctx *ParserContext) current() *Token {
	if ctx.cursor < len(ctx.tokens) {
		return ctx.tokens[ctx.cursor]
	}
	return nil

}
func (ctx *ParserContext) currentType() TokenType {
	if ctx.cursor < len(ctx.tokens) {
		return ctx.tokens[ctx.cursor].Type
	}

	return Eof

}

func NewParserContext(tokens []*Token, ast *Ast, cursor int) *ParserContext {
	return &ParserContext{
		tokens:       tokens,
		ast:          ast,
		cursor:       cursor,
		currentScope: ast.Scope,
	}
}

func (ctx *ParserContext) logError(message string, token *Token) {
	if token != nil {
		log.Printf("Error: %s at %s:%d:%d", message, token.Filename, token.Row+1, token.Col+1)
	} else {
		log.Printf("Error: %s (no token position available)", message)
	}
}

func (ctx *ParserContext) logErrorAtCurrent(message string) {
	ctx.logError(message, ctx.current())
}

func (ctx *ParserContext) logFatal(message string, token *Token) {
	if token != nil {
		log.Panicf("Error: %s at %s:%d:%d", message, token.Filename, token.Row+1, token.Col+1)
	} else {
		log.Panicf("Error: %s (no token position available)", message)
	}
}

func parseExpression(ctx *ParserContext) *Expression {
	return parseLogicalOr(ctx)
}

func parseBlock(ctx *ParserContext, parentScope *Scope) *Block {
	// Create new scope for this block
	blockScope := &Scope{
		Variables: make(map[string]*Variable),
		Parent:    parentScope,
	}

	// Save current scope and set new one
	previousScope := ctx.currentScope
	ctx.currentScope = blockScope

	block := Block{
		Scope: blockScope,
	}
	var statements []*Statement
	for {
		stmt := parseStatement(ctx)
		if stmt == nil {
			break
		}
		statements = append(statements, stmt)
	}
	block.Statements = statements

	// Restore previous scope
	ctx.currentScope = previousScope

	return &block
}

func parseAssignment(ctx *ParserContext) *Statement {

	statement := Statement{}
	statement.Type = AssignmentStmt
	ident := ctx.consumeAssert(Identifier)

	// Set LHS to the identifier expression
	statement.Lhs = &Expression{
		Type:  IdentifierExpr,
		Value: ident.Value,
		Token: ident,
		// ValueType will be determined during typechecking
	}
	statement.Token = ident

	if ctx.peek() == nil {
		ctx.logError("Unreachable - expected token after identifier", nil)
		return nil
	}

	// Variable Declaration
	if ctx.peek().Type == Colon {
		_ = ctx.consumeAssert(Colon)
		varType := ctx.consume()
		if varType == nil || !varType.Type.IsValidReturnType() {
			ctx.logError(fmt.Sprintf("Expected variable type after identifier '%s' but got '%s'", ident.Value, varType.Value), varType)
			return nil
		}

		statement.VarType = varType.Type
	}

	// Assignment
	if ctx.peek().Type == Assign {
		_ = ctx.consumeAssert(Assign)
		rhs := parseExpression(ctx)
		if rhs == nil {
			ctx.logError(fmt.Sprintf("Invalid right-hand side during assignment of variable '%s'", ident.Value), ctx.current())
			return nil
		}

		statement.Rhs = rhs
	}

	return &statement
}

func parseFunctionCall(ctx *ParserContext) *Expression {

	functionCall := Expression{}
	functionCall.Type = CallExpr
	ident := ctx.consumeAssert(Identifier)
	functionCall.Value = ident.Value
	functionCall.Token = ident
	_ = ctx.consumeAssert(LParen)
	// Parameter Parsing
	for {
		// End of Parameters
		if ctx.peek().Type == RParen {
			break
		}

		paramExpr := parseExpression(ctx)
		if paramExpr == nil {
			ctx.logError(fmt.Sprintf("Failed to parse parameters for call to function '%s'", ident.Value), ctx.current())
			return nil
		}
		functionCall.Args = append(functionCall.Args, paramExpr)
		_ = ctx.consumeIf(Comma)

	}
	_ = ctx.consumeAssert(RParen)
	return &functionCall

}
func parseStatement(ctx *ParserContext) *Statement {

	statement := Statement{}

	currentToken := ctx.peek()
	if currentToken == nil {
		return nil
	}

	switch currentToken.Type {

	case If:
		ifToken := ctx.consumeAssert(If)
		conditional := parseExpression(ctx)
		if conditional == nil {
			ctx.logFatal("Expected condition after if", ctx.current())
		}
		statement.Conditional = conditional
		_ = ctx.consumeAssert(LBrace)
		block := parseBlock(ctx, ctx.currentScope)
		_ = ctx.consumeAssert(RBrace)

		if block == nil {
			ctx.logErrorAtCurrent("failed to parse body of if statement")
			return nil
		}

		statement.Type = IfStmt
		statement.Block = block
		statement.Token = ifToken

		if ctx.peek() != nil && ctx.peek().Type == Else {
			_ = ctx.consumeAssert(Else)

			_ = ctx.consumeAssert(LBrace)
			elseblock := parseBlock(ctx, ctx.currentScope)
			_ = ctx.consumeAssert(RBrace)

			if elseblock == nil {
				ctx.logErrorAtCurrent("failed to parse body of else in if statement")
				return nil
			}

			statement.ElseBlock = elseblock
		}

	case While:
		_ = ctx.consumeAssert(While)
		conditional := parseExpression(ctx)
		if conditional == nil {
			ctx.logFatal("Expected condition after while", ctx.current())
		}
		statement.Conditional = conditional
		_ = ctx.consumeAssert(LBrace)
		block := parseBlock(ctx, ctx.currentScope)
		_ = ctx.consumeAssert(RBrace)

		if block == nil {
			ctx.logErrorAtCurrent("failed to parse body of while statement")
			return nil
		}

		statement.Type = WhileStmt
		statement.Block = block

	case Emit:
		_ = ctx.consumeAssert(Emit)
		rhs := parseExpression(ctx)
		if rhs == nil {
			ctx.logErrorAtCurrent("Expected expression after emit")
			return nil
		}
		statement.Rhs = rhs
		statement.Type = EmitStmt
	case Return:
		_ = ctx.consumeAssert(Return)
		rhs := parseExpression(ctx)
		if rhs == nil {
			ctx.logFatal("Expected expression after return", ctx.current())
		}
		statement.Rhs = rhs
		statement.Type = ReturnStmt

	// More complex case, could be variable declaration/assignments, function calls, etc
	case Identifier:

		// We don't consume because parseFunctionCall and parseAssignment both want to see the identifier
		ident := ctx.peek()
		if ident == nil || ident.Type != Identifier {
			ctx.logErrorAtCurrent("This should never be reached")
			return nil
		}
		if ctx.cursor+1 >= len(ctx.tokens) {
			ctx.logError(fmt.Sprintf("Dangling Identifier '%s'", ident.Value), ident)
			return nil
		}

		t := ctx.tokens[ctx.cursor+1].Type

		switch t {
		case Assign, Colon:
			// Variable Assignment / Declaration
			assignStmt := parseAssignment(ctx)
			if assignStmt == nil {
				ctx.logError(fmt.Sprintf("Expected Assignment after identifier '%s'", ident.Value), ident)
				return nil
			}

			return assignStmt

		case LParen:

			statement.Type = ExpressionStmt
			// Function Call
			functionCall := parseFunctionCall(ctx)
			if functionCall == nil {
				ctx.logError(fmt.Sprintf("Expected (...) after identifier '%s' representing function call", ident.Value), ident)
				return nil
			}

			statement.Rhs = functionCall

		default:
			// Standalone identifier or part of expression - parse as expression statement
			expr := parseExpression(ctx)
			if expr == nil {
				ctx.logError(fmt.Sprintf("Failed to parse expression starting with identifier '%s'", ident.Value), ident)
				return nil
			}
			statement.Type = ExpressionStmt
			statement.Rhs = expr
		}

	case RBrace:
		// End of a block, just return nil
		return nil
	default:

		// Dangling Expressions
		expr := parseExpression(ctx)
		if expr == nil {
			ctx.logFatal(fmt.Sprintf("Unknown Token with value '%s' in parseStatement", currentToken.Value), currentToken)
		}

		statement.Type = ExpressionStmt
		statement.Rhs = expr

	}
	return &statement

}

func parseFunctionDefinition(ctx *ParserContext) *Function {

	functionDefinition := Function{}
	_ = ctx.consumeAssert(Define)
	functionIdent := ctx.consumeAssert(Identifier)
	functionDefinition.Name = functionIdent.Value
	_ = ctx.consumeAssert(LParen)

	// Parameter Parsing
	for {
		// End of Parameters
		if ctx.peek().Type == RParen {
			break
		}

		paramIdent := ctx.consumeAssert(Identifier)
		functionDefinition.Parameters = append(functionDefinition.Parameters, paramIdent.Value)

		_ = ctx.consumeAssert(Colon)

		paramType := ctx.consume()
		if paramType == nil || !paramType.Type.IsValidReturnType() {
			ctx.logError(fmt.Sprintf("Expected parameter type after parameter '%s' but got '%s'", paramIdent.Value, paramType.Value), paramType)
			return nil
		}
		functionDefinition.ParameterTypes = append(functionDefinition.ParameterTypes, paramType.Type)

		_ = ctx.consumeIf(Comma)

	}
	_ = ctx.consumeAssert(RParen)

	// Parse return type
	_ = ctx.consumeAssert(Colon)
	returnType := ctx.consume()
	if returnType == nil || !returnType.Type.IsValidReturnType() {
		ctx.logError(fmt.Sprintf("Expected return type after function '%s' but got '%s'", functionIdent.Value, returnType.Value), returnType)
		return nil
	}
	functionDefinition.ReturnType = returnType.Type

	// Parse Body
	_ = ctx.consumeAssert(LBrace)
	block := parseBlock(ctx, ctx.ast.Scope)
	functionDefinition.Block = block
	if block == nil {
		ctx.logError(fmt.Sprintf("failed to parse body of function definition for function '%s'", functionIdent.Value), ctx.current())
		return nil
	}
	_ = ctx.consumeAssert(RBrace)

	return &functionDefinition
}

func Parse(tokens []*Token) (*Ast, error) {

	fmt.Println("Parsing")
	ast := NewAst(tokens)

	// Initialize global scope
	ast.Scope = &Scope{
		Variables: make(map[string]*Variable),
		Parent:    nil,
	}

	if len(tokens) == 0 {
		return ast, nil
	}
	var ctx *ParserContext = NewParserContext(tokens, ast, 0)

	for ctx.cursor < len(ctx.tokens) {
		if ctx.currentType() != Define {
			ctx.logFatal(fmt.Sprintf("Expected 'define' but got '%s'", ctx.currentType()), ctx.current())
		}
		functionDefinition := parseFunctionDefinition(ctx)
		if functionDefinition == nil {
			ctx.logFatal("could not parse function def", ctx.current())
		}
		ast.Functions[functionDefinition.Name] = functionDefinition
	}

	return ast, nil
}
func parseLogicalOr(ctx *ParserContext) *Expression {
	left := parseLogicalAnd(ctx)
	if left == nil {
		return nil
	}

	for ctx.peek() != nil && ctx.peek().Type == LogicalOr {
		op := ctx.consume()
		right := parseLogicalAnd(ctx)
		left = &Expression{
			Type:     BinaryExpr,
			Operator: op.Type,
			Lhs:      left,
			Rhs:      right,
			Token:    op,
		}
	}

	return left
}

func parseLogicalAnd(ctx *ParserContext) *Expression {
	left := parseRelational(ctx)
	if left == nil {
		return nil
	}

	for ctx.peek() != nil && ctx.peek().Type == LogicalAnd {
		op := ctx.consume()
		right := parseRelational(ctx)
		left = &Expression{
			Type:     BinaryExpr,
			Operator: op.Type,
			Lhs:      left,
			Rhs:      right,
			Token:    op,
		}
	}

	return left
}

func parseRelational(ctx *ParserContext) *Expression {
	left := parseAdditive(ctx)
	if left == nil {
		return nil
	}

	for ctx.peek() != nil && isRelationalOp(ctx.peek().Type) {
		op := ctx.consume()
		right := parseAdditive(ctx)
		left = &Expression{
			Type:     BinaryExpr,
			Operator: op.Type,
			Lhs:      left,
			Rhs:      right,
			Token:    op,
		}
	}

	return left
}

func parseAdditive(ctx *ParserContext) *Expression {
	left := parseTerm(ctx)
	if left == nil {
		return nil
	}

	for ctx.peek() != nil && (ctx.peek().Type == Plus || ctx.peek().Type == Minus) {
		op := ctx.consume()
		right := parseTerm(ctx)
		left = &Expression{
			Type:     BinaryExpr,
			Operator: op.Type,
			Lhs:      left,
			Rhs:      right,
			Token:    op,
		}
	}

	return left
}

func parseTerm(ctx *ParserContext) *Expression {
	left := parseFactor(ctx)
	if left == nil {
		return nil
	}

	for ctx.peek() != nil && (ctx.peek().Type == Multiply || ctx.peek().Type == Divide) {
		op := ctx.consume()
		right := parseFactor(ctx)
		left = &Expression{
			Type:     BinaryExpr,
			Operator: op.Type,
			Lhs:      left,
			Rhs:      right,
			Token:    op,
		}
	}

	return left
}

func parseFactor(ctx *ParserContext) *Expression {
	// Handle unary operators
	if ctx.peek() != nil && ctx.peek().Type == Minus {
		op := ctx.consume()
		return &Expression{
			Type:     UnaryExpr,
			Operator: op.Type,
			Rhs:      parseFactor(ctx),
		}
	}

	return parseAtom(ctx)
}

func parseAtom(ctx *ParserContext) *Expression {
	token := ctx.peek()
	if token == nil {
		return nil
	}

	switch token.Type {
	case IntLiteral, FloatLiteral, StringLiteral, BoolLiteral:
		ctx.consume()
		var valueType TokenType
		switch token.Type {
		case IntLiteral:
			valueType = Int
		case FloatLiteral:
			valueType = Float
		case StringLiteral:
			valueType = String
		case BoolLiteral:
			valueType = Bool
		}
		return &Expression{
			Type:      LiteralExpr,
			Value:     token.Value,
			ValueType: valueType,
			Token:     token,
		}

	case Identifier:
		// Check for function call first
		if ctx.cursor+1 < len(ctx.tokens) && ctx.tokens[ctx.cursor+1].Type == LParen {
			return parseFunctionCall(ctx)
		}

		ctx.consume()
		return &Expression{
			Type:  IdentifierExpr,
			Value: token.Value,
			Token: token,
			// ValueType will be determined during typechecking
		}

	case LParen:
		ctx.consume()
		expr := parseExpression(ctx)
		ctx.consumeAssert(RParen)
		return expr

	default:
		return nil
	}
}

func isRelationalOp(t TokenType) bool {
	return t == Equal || t == NotEqual ||
		t == LessThan || t == LessEqual ||
		t == GreaterThan || t == GreaterEqual
}
