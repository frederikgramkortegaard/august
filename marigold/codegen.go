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

	// Memory allocation for string literals and other heap data
	NextMemoryAddr   uint64              // Next available memory address

	// String table for efficient string literal handling
	StringTable      map[string]uint64   // Maps string content to memory address
	StringLiterals   []string            // Ordered list of unique string literals
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

// RegisterStringLiteral adds a string literal to the string table if not already present
// Returns the memory address where the string will be stored
func (ctx *CodegenContext) RegisterStringLiteral(stringValue string) uint64 {
	// Check if string is already in table
	if addr, exists := ctx.StringTable[stringValue]; exists {
		return addr
	}

	// Calculate memory needed for this string
	stringBytes := []byte(stringValue)
	byteLength := uint64(len(stringBytes))
	chunksNeeded := (byteLength + 31) / 32 // ceil(byteLength / 32)

	// Allocate memory address for this string
	stringAddr := ctx.NextMemoryAddr
	ctx.NextMemoryAddr += 1 + chunksNeeded // length + chunks

	// Add to string table
	ctx.StringTable[stringValue] = stringAddr
	ctx.StringLiterals = append(ctx.StringLiterals, stringValue)

	return stringAddr
}

// CollectStringLiterals walks through an AST and registers all string literals without generating code
func (ctx *CodegenContext) CollectStringLiterals(ast *Ast) {
	for _, function := range ast.Functions {
		ctx.collectStringLiteralsFromFunction(function)
	}
}

func (ctx *CodegenContext) collectStringLiteralsFromFunction(function *Function) {
	if function.Block != nil {
		ctx.collectStringLiteralsFromBlock(function.Block)
	}
}

func (ctx *CodegenContext) collectStringLiteralsFromBlock(block *Block) {
	for _, stmt := range block.Statements {
		ctx.collectStringLiteralsFromStatement(stmt)
	}
}

func (ctx *CodegenContext) collectStringLiteralsFromStatement(stmt *Statement) {
	switch stmt.Type {
	case ExpressionStmt:
		if stmt.Rhs != nil {
			ctx.collectStringLiteralsFromExpression(stmt.Rhs)
		}
	case AssignmentStmt:
		if stmt.Rhs != nil {
			ctx.collectStringLiteralsFromExpression(stmt.Rhs)
		}
	case ReturnStmt:
		if stmt.Rhs != nil {
			ctx.collectStringLiteralsFromExpression(stmt.Rhs)
		}
	case IfStmt:
		if stmt.Conditional != nil {
			ctx.collectStringLiteralsFromExpression(stmt.Conditional)
		}
		if stmt.Block != nil {
			ctx.collectStringLiteralsFromBlock(stmt.Block)
		}
		if stmt.ElseBlock != nil {
			ctx.collectStringLiteralsFromBlock(stmt.ElseBlock)
		}
	case WhileStmt:
		if stmt.Conditional != nil {
			ctx.collectStringLiteralsFromExpression(stmt.Conditional)
		}
		if stmt.Block != nil {
			ctx.collectStringLiteralsFromBlock(stmt.Block)
		}
	}
}

func (ctx *CodegenContext) collectStringLiteralsFromExpression(expr *Expression) {
	if expr == nil {
		return
	}

	switch expr.Type {
	case LiteralExpr:
		if expr.ValueType == String {
			stringValue := expr.Value.(string)
			ctx.RegisterStringLiteral(stringValue)
		}
	case BinaryExpr:
		ctx.collectStringLiteralsFromExpression(expr.Lhs)
		ctx.collectStringLiteralsFromExpression(expr.Rhs)
	case UnaryExpr:
		ctx.collectStringLiteralsFromExpression(expr.Rhs)
	case CallExpr:
		for _, arg := range expr.Args {
			ctx.collectStringLiteralsFromExpression(arg)
		}
	case ArrayLiteral:
		for _, element := range expr.Args {
			ctx.collectStringLiteralsFromExpression(element)
		}
	case IndexExpr:
		ctx.collectStringLiteralsFromExpression(expr.Lhs)
		ctx.collectStringLiteralsFromExpression(expr.Rhs)
	}
}

// GenerateStringInitialization generates bytecode to initialize all string literals in memory
func (ctx *CodegenContext) GenerateStringInitialization() {
	for _, stringValue := range ctx.StringLiterals {
		stringAddr := ctx.StringTable[stringValue]
		stringBytes := []byte(stringValue)
		byteLength := uint64(len(stringBytes))

		// Calculate number of 32-byte chunks needed
		chunksNeeded := (byteLength + 31) / 32 // ceil(byteLength / 32)

		// Store byte length at first position
		ctx.EmitPush(byteLength)
		ctx.EmitPush(stringAddr)
		ctx.Emit(types.MSTORE)

		// Store string data in 32-byte chunks
		for i := uint64(0); i < chunksNeeded; i++ {
			chunkStart := i * 32
			chunkEnd := chunkStart + 32
			if chunkEnd > byteLength {
				chunkEnd = byteLength
			}

			// Extract chunk and pad to 32 bytes
			chunk := make([]byte, 32)
			copy(chunk, stringBytes[chunkStart:chunkEnd])

			// Convert chunk to big integer (treating as big-endian bytes)
			chunkValue := fmt.Sprintf("0x%x", chunk)

			// Store chunk at memory address
			ctx.EmitPush(chunkValue)
			ctx.EmitPush(stringAddr + 1 + i)
			ctx.Emit(types.MSTORE)
		}
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
		// Register string literal in string table and get its address
		stringValue := expr.Value.(string)
		stringAddr := ctx.RegisterStringLiteral(stringValue)

		// Push pointer to string (the starting address)
		ctx.EmitPush(stringAddr)
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

	// Check if it's a blockchain context variable
	if blockchainVar := GetBlockchainContextVariable(varName); blockchainVar != nil {
		switch varName {
		case "@caller":
			ctx.Emit(types.CALLER)
		case "@address":
			ctx.Emit(types.ADDRESS)
		case "@balance":
			ctx.Emit(types.BALANCE)
		case "@origin":
			ctx.Emit(types.ORIGIN)
		case "@gasprice":
			ctx.Emit(types.GASPRICE)
		case "@callvalue":
			ctx.Emit(types.CALLVALUE)
		case "@timestamp":
			ctx.Emit(types.TIMESTAMP)
		case "@difficulty":
			ctx.Emit(types.DIFFICULTY)
		case "@coinbase":
			ctx.Emit(types.COINBASE)
		case "@height":
			ctx.Emit(types.HEIGHT)
		case "@gaslimit":
			ctx.Emit(types.GASLIMIT)
		case "@tsxdata":
			ctx.Emit(types.TSXDATA)
		default:
			ctx.logFatalWithToken(fmt.Sprintf("Unknown blockchain context variable: %s", varName), expr.Token)
		}
		return
	}

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
		// emit(value) - automatically use EMITSTR for strings, EMIT for others
		if len(expr.Args) != 1 {
			ctx.logFatalWithToken("emit() expects exactly 1 argument", expr.Token)
			return
		}
		GenerateExpression(ctx, expr.Args[0])
		// Check if argument is a string
		if isStringExpression(ctx, expr.Args[0]) {
			ctx.Emit(types.EMITSTR)
		} else {
			ctx.Emit(types.EMIT)
		}
		return
	}

	if funcName == "len" {
		// len(value) - automatically use STRLEN for strings
		if len(expr.Args) != 1 {
			ctx.logFatalWithToken("len() expects exactly 1 argument", expr.Token)
			return
		}
		GenerateExpression(ctx, expr.Args[0])
		// Check if argument is a string
		if isStringExpression(ctx, expr.Args[0]) {
			ctx.Emit(types.STRLEN)
		} else {
			ctx.logFatalWithToken("len() currently only supports strings", expr.Token)
		}
		return
	}

	if funcName == "int" {
		// int(value) - convert string to integer
		if len(expr.Args) != 1 {
			ctx.logFatalWithToken("int() expects exactly 1 argument", expr.Token)
			return
		}
		GenerateExpression(ctx, expr.Args[0])
		// Check if argument is a string
		if isStringExpression(ctx, expr.Args[0]) {
			ctx.Emit(types.STRTOINT)
		} else {
			ctx.logFatalWithToken("int() currently only supports string arguments", expr.Token)
		}
		return
	}

	if funcName == "string" {
		// string(value) - convert integer to string
		if len(expr.Args) != 1 {
			ctx.logFatalWithToken("string() expects exactly 1 argument", expr.Token)
			return
		}
		GenerateExpression(ctx, expr.Args[0])
		// Check if argument is an integer
		if !isStringExpression(ctx, expr.Args[0]) {
			ctx.Emit(types.INTTOSTR)
		} else {
			ctx.logFatalWithToken("string() currently only supports integer arguments", expr.Token)
		}
		return
	}

	// String operations
	if funcName == "emit_str" {
		if len(expr.Args) != 1 {
			ctx.logFatalWithToken("emit_str() expects exactly 1 argument", expr.Token)
			return
		}
		GenerateExpression(ctx, expr.Args[0])
		ctx.Emit(types.EMITSTR)
		return
	}

	if funcName == "str_concat" {
		if len(expr.Args) != 2 {
			ctx.logFatalWithToken("str_concat() expects exactly 2 arguments", expr.Token)
			return
		}
		GenerateExpression(ctx, expr.Args[0])
		GenerateExpression(ctx, expr.Args[1])
		ctx.Emit(types.STRCONCAT)
		return
	}

	if funcName == "str_len" {
		if len(expr.Args) != 1 {
			ctx.logFatalWithToken("str_len() expects exactly 1 argument", expr.Token)
			return
		}
		GenerateExpression(ctx, expr.Args[0])
		ctx.Emit(types.STRLEN)
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

	// Check if this is string indexing
	if isStringExpression(ctx, expr.Lhs) {
		// Generate the string expression (pushes string address)
		GenerateExpression(ctx, expr.Lhs)
		// Generate the index expression (pushes index)
		GenerateExpression(ctx, expr.Rhs)
		// Emit STRINDEX to get character
		ctx.Emit(types.STRINDEX)
		return
	}

	// Other indexing not yet supported
	ctx.logFatalWithToken("Index expressions only supported for persistent[], memory[], and strings", expr.Token)
}

func generateSliceExpr(ctx *CodegenContext, expr *Expression) {
	// Check if this is string slicing
	if isStringExpression(ctx, expr.Lhs) {
		// STRSLICE expects: [string_addr, start, end] on stack

		// Generate the string expression (pushes string address)
		GenerateExpression(ctx, expr.Lhs)

		// Generate start index (or 0 if nil)
		if expr.SliceStart != nil {
			GenerateExpression(ctx, expr.SliceStart)
		} else {
			ctx.EmitPush(0) // Default start = 0
		}

		// Generate end index (or use 999999 to indicate "use string length")
		if expr.SliceEnd != nil {
			GenerateExpression(ctx, expr.SliceEnd)
		} else {
			ctx.EmitPush(999999) // Special value: 999999 means "use string length"
		}

		// Emit STRSLICE: expects [string_addr, start, end]
		ctx.Emit(types.STRSLICE)
		return
	}

	// Array slicing not yet supported
	ctx.logFatalWithToken("Slice expressions only supported for strings", expr.Token)
}

func isStringExpression(ctx *CodegenContext, expr *Expression) bool {
	// Check if it's a string literal
	if expr.Type == LiteralExpr && expr.ValueType == String {
		return true
	}

	// Check if it's a variable with string type
	if expr.Type == IdentifierExpr {
		varName := expr.Value.(string)
		// Look up in current scope and parent scopes
		scope := ctx.CurrentScope
		for scope != nil {
			if variable, exists := scope.Variables[varName]; exists {
				// Check if the type is StringType
				if variable.Type.Equals(StringType) {
					return true
				}
				break
			}
			scope = scope.Parent
		}
	}

	return false
}

func generateBinaryExpression(ctx *CodegenContext, expr *Expression) {
	// Generate left operand (pushes result onto stack)
	GenerateExpression(ctx, expr.Lhs)

	// Generate right operand (pushes result onto stack)
	GenerateExpression(ctx, expr.Rhs)

	// Check if we're doing string concatenation
	isStringConcat := false
	if expr.Operator == Plus {
		// Check if both operands are strings (literals or variables)
		if isStringExpression(ctx, expr.Lhs) && isStringExpression(ctx, expr.Rhs) {
			isStringConcat = true
		}
	}

	// Emit the operation (pops two values, pushes result)
	switch expr.Operator {
	case Plus:
		if isStringConcat {
			ctx.Emit(types.STRCONCAT)
		} else {
			ctx.Emit(types.ADD)
		}
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
			// Generate the key expression (string address for string literals)
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
		NextMemoryAddr:  0, // Start memory allocation at address 0
		StringTable:     make(map[string]uint64),
		StringLiterals:  make([]string, 0),
	}

	// First pass: Collect all string literals without generating code
	ctx.CollectStringLiterals(ast)

	// IC=0: Jump to init path (string_init + init_function)
	ctx.EmitPush(0) // Placeholder for init path address
	ctx.Emit(types.JUMP)

	// IC=1: Jump to call path (string_init + call_function)
	ctx.EmitPush(0) // Placeholder for call path address
	ctx.Emit(types.JUMP)

	// Init path: String initialization + jump to init function
	initPathStart := len(ctx.Instructions)
	ctx.GenerateStringInitialization()
	// Add jump to init function (will be patched later)
	initJumpPlaceholder := len(ctx.Instructions)
	ctx.EmitPush(0) // Placeholder for init function address
	ctx.Emit(types.JUMP)

	// Call path: String initialization + jump to call function
	callPathStart := len(ctx.Instructions)
	ctx.GenerateStringInitialization()
	// Add jump to call function (will be patched later)
	callJumpPlaceholder := len(ctx.Instructions)
	ctx.EmitPush(0) // Placeholder for call function address
	ctx.Emit(types.JUMP)

	// Generate all function definitions in correct order (init first, then others, then call)
	// This ensures functions are defined before they're referenced
	if initFunc, ok := ast.Functions["init"]; ok {
		GenerateFunctionDefinition(ctx, initFunc)
	}

	// Generate other functions (not init or call)
	for name, function := range ast.Functions {
		if name != "init" && name != "call" {
			GenerateFunctionDefinition(ctx, function)
		}
	}

	// Generate call function last (so it can reference other functions)
	if callFunc, ok := ast.Functions["call"]; ok {
		GenerateFunctionDefinition(ctx, callFunc)
	}

	// Patch all entry points and function jumps
	if initAddr, ok := ctx.FunctionIndex["init"]; ok {
		// IC=0 jumps to init path (string init before init function)
		initPathHex := fmt.Sprintf("0x%x", initPathStart)
		ctx.Instructions[0].Value = &initPathHex

		// Init path jumps to actual init function
		initFuncHex := fmt.Sprintf("0x%x", initAddr)
		ctx.Instructions[initJumpPlaceholder].Value = &initFuncHex
	} else {
		ctx.logFatal("init function not found - required for contracts")
	}

	if callAddr, ok := ctx.FunctionIndex["call"]; ok {
		// IC=1 jumps to call path (string init before call function)
		callPathHex := fmt.Sprintf("0x%x", callPathStart)
		ctx.Instructions[2].Value = &callPathHex

		// Call path jumps to actual call function
		callFuncHex := fmt.Sprintf("0x%x", callAddr)
		ctx.Instructions[callJumpPlaceholder].Value = &callFuncHex
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
