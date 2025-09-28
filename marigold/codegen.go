package marigold

import (
	"august/types"
	"fmt"
	"log"
)

type CodegenContext struct {
	Ast              *Ast
	CurrentFunction  *Function
	CurrentScope     *Scope
	Instructions     []types.Instruction // Accumulated bytecode instructions
	FunctionIndex    map[string]int      // Maps function name to instruction index

	// Frame-based locals (for function parameters and local variables)
	LocalOffsets     map[string]int      // Maps variable name to frame offset (within current function)
	NextLocalOffset  int                 // Next available local offset in current function

	// Loop context (for break/continue)
	LoopStack        []LoopContext       // Stack of nested loops

}

type LoopContext struct {
	ContinueAddr     int                 // Address to jump to for continue (loop start)
	BreakPatches     []int               // Instruction indices that need to be patched to jump to loop end
}

func (ctx *CodegenContext) logFatal(message string) {
	if ctx.CurrentFunction != nil {
		log.Panicf("Type error in function '%s': %s", ctx.CurrentFunction.Name, message)
	} else {
		log.Panicf("Type error: %s", message)
	}
}

func (ctx *CodegenContext) logFatalWithToken(message string, token *Token) {
	if token != nil {
		if ctx.CurrentFunction != nil {
			log.Panicf("Type error in function '%s': %s at %s:%d:%d", ctx.CurrentFunction.Name, message, token.Filename, token.Row+1, token.Col+1)
		} else {
			log.Panicf("Type error: %s at %s:%d:%d", message, token.Filename, token.Row+1, token.Col+1)
		}
	} else {
		ctx.logFatal(message)
	}
}

func (ctx *CodegenContext) Emit(opcode types.OPCODE) {
	ctx.Instructions = append(ctx.Instructions, types.MakeInstruction(opcode))
}

func (ctx *CodegenContext) EmitPush(value interface{}) {
	ctx.Instructions = append(ctx.Instructions, types.MakeInstructionWithValue(types.PUSH, value))
}

func GenerateExpression(ctx *CodegenContext, f *Expression) {
	switch f.Type {
	case LiteralExpr:
		generateLiteral(ctx, f)
	case BinaryExpr:
		generateBinaryExpression(ctx, f)
	case UnaryExpr:
		generateUnaryExpression(ctx, f)
	case IdentifierExpr:
		generateIdentifier(ctx, f)
	case CallExpr:
		generateCall(ctx, f)
	case ArrayLiteral:
		generateArrayLiteral(ctx, f)
	case MapLiteral:
		generateMapLiteral(ctx, f)
	case IndexExpr:
		generateIndexExpr(ctx, f)
	case SliceExpr:
		generateSliceExpr(ctx, f)
	default:
		ctx.logFatalWithToken("Unsupported expression type: "+string(f.Type), f.Token)
	}
}

func generateLiteral(ctx *CodegenContext, expr *Expression) {
	switch expr.ValueType {
	case Int:
		// Push integer value onto stack
		ctx.EmitPush(expr.Value)
	case Bool:
		// Push boolean as integer (true=1, false=0)
		if expr.Value.(bool) {
			ctx.EmitPush(1)
		} else {
			ctx.EmitPush(0)
		}
	case String:
		// TODO: String literals need special handling (heap allocation, etc)
		// For now, just push 0 as placeholder
		ctx.logFatalWithToken("String literals not yet supported", expr.Token)
	case Float:
		// TODO: Float literals need special handling (AVM uses 256-bit integers)
		// For now, just push 0 as placeholder
		ctx.logFatalWithToken("Float literals not yet supported", expr.Token)
	default:
		ctx.logFatalWithToken("Unsupported literal type: "+string(expr.ValueType), expr.Token)
	}
}

func generateIdentifier(ctx *CodegenContext, expr *Expression) {
	varName := expr.Value.(string)

	// Check if it's a local variable (in current function)
	if ctx.CurrentFunction != nil {
		if offset, exists := ctx.LocalOffsets[varName]; exists {
			// Load from frame: LOAD_LOCAL offset
			ctx.EmitPush(offset)
			ctx.Emit(types.LOAD_LOCAL)
			return
		}
	}

	// Variable not found
	ctx.logFatalWithToken("Undefined variable: "+varName, expr.Token)
}

func generateCall(ctx *CodegenContext, expr *Expression) {
	// Function name is in expr.Value
	funcName := expr.Value.(string)

	// Check for builtin functions
	if funcName == "emit" {
		// emit(value) - pushes value and calls EMIT opcode
		if len(expr.Args) != 1 {
			ctx.logFatalWithToken("emit() expects exactly 1 argument", expr.Token)
			return
		}
		GenerateExpression(ctx, expr.Args[0])
		ctx.Emit(types.EMIT)
		return
	}

	// Type conversion functions (placeholders for now - just pass through the value)
	if funcName == "string" || funcName == "int" || funcName == "float" {
		if len(expr.Args) != 1 {
			ctx.logFatalWithToken(funcName+"() expects exactly 1 argument", expr.Token)
			return
		}
		// Just generate the argument expression (no conversion yet)
		GenerateExpression(ctx, expr.Args[0])
		return
	}

	// Look up function address
	funcAddr, exists := ctx.FunctionIndex[funcName]
	if !exists {
		ctx.logFatalWithToken("Undefined function: "+funcName, expr.Token)
		return
	}

	// Push arguments in order (they will be at frame offsets 0, 1, 2, ...)
	for _, arg := range expr.Args {
		GenerateExpression(ctx, arg)
	}

	// CALL instruction: pushes return address, sets frame pointer, jumps to function
	ctx.EmitPush(funcAddr)
	ctx.Emit(types.CALL)

	// After return, result is on top of stack
}

func generateArrayLiteral(ctx *CodegenContext, expr *Expression) {
	// TODO: Array literals need heap allocation and memory management
	// For now, not supported
	ctx.logFatalWithToken("Array literals not yet supported", expr.Token)
}

func generateMapLiteral(ctx *CodegenContext, expr *Expression) {
	// TODO: Map literals need heap allocation and hash table implementation
	// For now, not supported
	ctx.logFatalWithToken("Map literals not yet supported", expr.Token)
}

func generateIndexExpr(ctx *CodegenContext, expr *Expression) {
	// Check if this is persistent storage access
	if expr.Lhs.Type == IdentifierExpr && expr.Lhs.Value.(string) == "persistent" {
		// Generate the key expression
		GenerateExpression(ctx, expr.Rhs)
		// Emit PLOAD to load from persistent storage
		ctx.Emit(types.PLOAD)
		return
	}

	// Check if this is memory access
	if expr.Lhs.Type == IdentifierExpr && expr.Lhs.Value.(string) == "memory" {
		// Generate the index expression
		GenerateExpression(ctx, expr.Rhs)
		// Emit MLOAD to load from memory
		ctx.Emit(types.MLOAD)
		return
	}

	// Other indexing not yet supported
	ctx.logFatalWithToken("Index expressions only supported for persistent[] and memory[]", expr.Token)
}

func generateSliceExpr(ctx *CodegenContext, expr *Expression) {
	// TODO: Array slicing needs runtime support
	// For now, not supported
	ctx.logFatalWithToken("Slice expressions not yet supported", expr.Token)
}

func generateBinaryExpression(ctx *CodegenContext, expr *Expression) {
	// Generate left operand (pushes result onto stack)
	GenerateExpression(ctx, expr.Lhs)

	// Generate right operand (pushes result onto stack)
	GenerateExpression(ctx, expr.Rhs)

	// Emit the operation (pops two values, pushes result)
	switch expr.Operator {
	case Plus:
		ctx.Emit(types.ADD)
	case Minus:
		ctx.Emit(types.SUB)
	case Multiply:
		ctx.Emit(types.MUL)
	case Divide:
		ctx.Emit(types.DIV)
	case Equal:
		ctx.Emit(types.EQ)
	case NotEqual:
		// != is implemented as !(a == b) = !EQ
		ctx.Emit(types.EQ)
		ctx.Emit(types.ISZERO)
	case LessThan:
		ctx.Emit(types.LT)
	case GreaterThan:
		ctx.Emit(types.GT)
	case LessEqual:
		// <= is implemented as !(a > b)
		ctx.Emit(types.GT)
		ctx.Emit(types.ISZERO)
	case GreaterEqual:
		// >= is implemented as !(a < b)
		ctx.Emit(types.LT)
		ctx.Emit(types.ISZERO)
	case Modulo:
		ctx.Emit(types.MOD)
	default:
		ctx.logFatalWithToken("Unsupported binary operator: "+string(expr.Operator), expr.Token)
	}
}


func generateUnaryExpression(ctx *CodegenContext, expr *Expression) {
	// Generate the operand (pushes result onto stack)
	GenerateExpression(ctx, expr.Rhs)

	// Emit the operation
	switch expr.Operator {
	case Minus:
		// Negate: push 0, swap, subtract (0 - value)
		ctx.EmitPush(0)
		ctx.EmitPush(1) // Swap index
		ctx.Emit(types.SWAP)
		ctx.Emit(types.SUB)
	case Not:
		// Logical not: check if zero (0 -> 1, nonzero -> 0)
		ctx.Emit(types.ISZERO)
	default:
		ctx.logFatalWithToken("Unsupported unary operator: "+string(expr.Operator), expr.Token)
	}
}

func GenerateFunctionDefinition(ctx *CodegenContext, f *Function) {
	if _, ok := ctx.FunctionIndex[f.Name]; ok {
		// Already defined, ignore
		return
	}

	// Record function start address
	ctx.FunctionIndex[f.Name] = len(ctx.Instructions)

	// Backup current context
	backFunc := ctx.CurrentFunction
	backScope := ctx.CurrentScope
	backLocals := ctx.LocalOffsets
	backNextLocal := ctx.NextLocalOffset

	// Setup function context
	ctx.CurrentFunction = f
	ctx.CurrentScope = f.Block.Scope
	ctx.LocalOffsets = make(map[string]int)
	ctx.NextLocalOffset = 0

	// Parameters are already on the stack (pushed by caller)
	// Assign offsets to parameters (0, 1, 2, ...)
	for i, paramName := range f.Parameters {
		ctx.LocalOffsets[paramName] = i
	}
	ctx.NextLocalOffset = len(f.Parameters)

	// Generate function body
	generateBlock(ctx, f.Block)

	// Add RETURN instruction at end if last instruction wasn't already RETURN
	if len(ctx.Instructions) == 0 || ctx.Instructions[len(ctx.Instructions)-1].Opcode != types.RETURN {
		// Push default return value (0 for int, false for bool, etc)
		ctx.EmitPush(0)
		ctx.Emit(types.RETURN)
	}

	// Restore context
	ctx.CurrentFunction = backFunc
	ctx.CurrentScope = backScope
	ctx.LocalOffsets = backLocals
	ctx.NextLocalOffset = backNextLocal
}

func generateBlock(ctx *CodegenContext, block *Block) {
	// Save and switch scope
	backScope := ctx.CurrentScope
	ctx.CurrentScope = block.Scope

	// Generate each statement
	for _, stmt := range block.Statements {
		generateStatement(ctx, stmt)
	}

	// Restore scope
	ctx.CurrentScope = backScope
}

func generateStatement(ctx *CodegenContext, stmt *Statement) {
	switch stmt.Type {
	case AssignmentStmt:
		generateAssignment(ctx, stmt)
	case ReturnStmt:
		generateReturn(ctx, stmt)
	case IfStmt:
		generateIf(ctx, stmt)
	case WhileStmt:
		generateWhile(ctx, stmt)
	case BreakStmt:
		generateBreak(ctx, stmt)
	case ContinueStmt:
		generateContinue(ctx, stmt)
	case ExpressionStmt:
		// Just evaluate the expression (result left on stack, then popped)
		GenerateExpression(ctx, stmt.Rhs)
		ctx.Emit(types.POP) // Pop result since it's not used
	default:
		ctx.logFatalWithToken("Unsupported statement type: "+string(stmt.Type), stmt.Token)
	}
}

func generateAssignment(ctx *CodegenContext, stmt *Statement) {
	// Generate RHS expression (leaves value on stack)
	GenerateExpression(ctx, stmt.Rhs)

	// Check if LHS is an index expression (e.g. persistent["key"] = value)
	if stmt.Lhs.Type == IndexExpr {
		// Check if this is persistent storage assignment
		if stmt.Lhs.Lhs.Type == IdentifierExpr && stmt.Lhs.Lhs.Value.(string) == "persistent" {
			// Generate the key expression (pushes key onto stack)
			GenerateExpression(ctx, stmt.Lhs.Rhs)
			// Stack now has: [value, key]
			// Emit PSTORE to store value with key
			ctx.Emit(types.PSTORE)
			return
		}

		// Check if this is memory assignment
		if stmt.Lhs.Lhs.Type == IdentifierExpr && stmt.Lhs.Lhs.Value.(string) == "memory" {
			// Generate the index expression (pushes index onto stack)
			GenerateExpression(ctx, stmt.Lhs.Rhs)
			// Stack now has: [value, index]
			// Emit MSTORE to store value at index
			ctx.Emit(types.MSTORE)
			return
		}

		ctx.logFatalWithToken("Index assignment only supported for persistent[] and memory[]", stmt.Token)
		return
	}

	// Regular variable assignment
	varName := stmt.Lhs.Value.(string)

	// Check if variable already exists
	if offset, exists := ctx.LocalOffsets[varName]; exists {
		// Existing local - store to it
		ctx.EmitPush(offset)
		ctx.Emit(types.STORE_LOCAL)
	} else {
		// New local variable - allocate next offset
		offset := ctx.NextLocalOffset
		ctx.LocalOffsets[varName] = offset
		ctx.NextLocalOffset++

		// Store to new local
		ctx.EmitPush(offset)
		ctx.Emit(types.STORE_LOCAL)
	}
}

func generateReturn(ctx *CodegenContext, stmt *Statement) {
	// Generate return value expression (leaves value on stack)
	if stmt.Rhs != nil {
		GenerateExpression(ctx, stmt.Rhs)
	} else {
		// No return value, push 0
		ctx.EmitPush(0)
	}

	// RETURN instruction will pop return address and jump back
	ctx.Emit(types.RETURN)
}

func generateIf(ctx *CodegenContext, stmt *Statement) {
	// Generate condition expression
	GenerateExpression(ctx, stmt.Conditional)

	// ISZERO to invert condition (JUMPC jumps if true/1)
	ctx.Emit(types.ISZERO)

	// Reserve space for JUMPC instruction (jump to else/end if condition is false)
	jumpcIndex := len(ctx.Instructions)
	ctx.EmitPush(0) // Placeholder for jump address
	ctx.Emit(types.JUMPC)

	// Generate then block
	generateBlock(ctx, stmt.Block)

	if stmt.ElseBlock != nil {
		// Reserve space for JUMP to skip else block
		jumpIndex := len(ctx.Instructions)
		ctx.EmitPush(0) // Placeholder
		ctx.Emit(types.JUMP)

		// Patch JUMPC to jump here (else block start)
		elseStart := len(ctx.Instructions)
		hexValue := fmt.Sprintf("0x%x", elseStart)
		ctx.Instructions[jumpcIndex].Value = &hexValue

		// Generate else block
		generateBlock(ctx, stmt.ElseBlock)

		// Patch JUMP to jump here (after else)
		endIndex := len(ctx.Instructions)
		hexValue2 := fmt.Sprintf("0x%x", endIndex)
		ctx.Instructions[jumpIndex].Value = &hexValue2
	} else {
		// No else block - patch JUMPC to jump to end
		endIndex := len(ctx.Instructions)
		hexValue := fmt.Sprintf("0x%x", endIndex)
		ctx.Instructions[jumpcIndex].Value = &hexValue
	}
}

func generateWhile(ctx *CodegenContext, stmt *Statement) {
	// Loop start - where continue jumps to
	loopStart := len(ctx.Instructions)

	// Generate condition expression
	GenerateExpression(ctx, stmt.Conditional)

	// ISZERO to invert condition (JUMPC jumps if true/1, we want to exit if false)
	ctx.Emit(types.ISZERO)

	// Reserve space for JUMPC to exit loop
	jumpcIndex := len(ctx.Instructions)
	ctx.EmitPush(0) // Placeholder
	ctx.Emit(types.JUMPC)

	// Push loop context
	loopCtx := LoopContext{
		ContinueAddr: loopStart,
		BreakPatches: make([]int, 0),
	}
	ctx.LoopStack = append(ctx.LoopStack, loopCtx)

	// Generate loop body
	generateBlock(ctx, stmt.Block)

	// Pop loop context
	loopCtx = ctx.LoopStack[len(ctx.LoopStack)-1]
	ctx.LoopStack = ctx.LoopStack[:len(ctx.LoopStack)-1]

	// Jump back to loop start
	ctx.EmitPush(loopStart)
	ctx.Emit(types.JUMP)

	// Patch JUMPC to jump here (loop end)
	loopEnd := len(ctx.Instructions)
	hexValue := fmt.Sprintf("0x%x", loopEnd)
	ctx.Instructions[jumpcIndex].Value = &hexValue

	// Patch all break statements to jump here
	for _, breakIdx := range loopCtx.BreakPatches {
		hexValue := fmt.Sprintf("0x%x", loopEnd)
		ctx.Instructions[breakIdx].Value = &hexValue
	}
}

func generateBreak(ctx *CodegenContext, stmt *Statement) {
	if len(ctx.LoopStack) == 0 {
		ctx.logFatalWithToken("Break statement outside of loop", stmt.Token)
		return
	}

	// Add a placeholder JUMP instruction
	jumpIndex := len(ctx.Instructions)
	ctx.EmitPush(0) // Placeholder - will be patched by generateWhile
	ctx.Emit(types.JUMP)

	// Record this instruction index for patching
	currentLoop := &ctx.LoopStack[len(ctx.LoopStack)-1]
	currentLoop.BreakPatches = append(currentLoop.BreakPatches, jumpIndex)
}

func generateContinue(ctx *CodegenContext, stmt *Statement) {
	if len(ctx.LoopStack) == 0 {
		ctx.logFatalWithToken("Continue statement outside of loop", stmt.Token)
		return
	}

	// Jump to loop start (condition check)
	currentLoop := ctx.LoopStack[len(ctx.LoopStack)-1]
	ctx.EmitPush(currentLoop.ContinueAddr)
	ctx.Emit(types.JUMP)
}

func CodegenWithContext(ast *Ast) *CodegenContext {
	ctx := &CodegenContext{
		Ast:             ast,
		Instructions:    make([]types.Instruction, 0),
		FunctionIndex:   make(map[string]int),
		LocalOffsets:    make(map[string]int),
		NextLocalOffset: 0,
		LoopStack:       make([]LoopContext, 0),
	}

	// Reserve space for entry point jumps (IC=0 and IC=1)
	// IC=0: JUMP to init
	// IC=1: JUMP to call
	ctx.EmitPush(0) // Placeholder for init address
	ctx.Emit(types.JUMP)
	ctx.EmitPush(0) // Placeholder for call address
	ctx.Emit(types.JUMP)

	// Generate all function definitions
	for _, function := range ast.Functions {
		GenerateFunctionDefinition(ctx, function)
	}

	// Patch the entry point jumps with actual addresses
	if initAddr, ok := ctx.FunctionIndex["init"]; ok {
		hexValue := fmt.Sprintf("0x%x", initAddr)
		ctx.Instructions[0].Value = &hexValue
	} else {
		ctx.logFatal("init function not found - required for contracts")
	}

	if callAddr, ok := ctx.FunctionIndex["call"]; ok {
		hexValue := fmt.Sprintf("0x%x", callAddr)
		ctx.Instructions[2].Value = &hexValue
	} else {
		ctx.logFatal("call function not found - required for contracts")
	}

	return ctx
}

func Codegen(ast *Ast) string {
	ctx := CodegenWithContext(ast)

	// Convert instructions to string format
	var output string
	for _, inst := range ctx.Instructions {
		if inst.Opcode == types.PUSH && inst.Value != nil {
			output += "PUSH " + *inst.Value + " "
		} else {
			output += inst.Opcode.String() + " "
		}
	}

	return output
}
