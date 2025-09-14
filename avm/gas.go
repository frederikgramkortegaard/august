package avm

var opcodeGasPrices = [...]uint8{
	NOOP:   1,  // Minimal cost for no-op
	PC:     1,  // Reading program counter
	PUSH:   3,  // Stack manipulation with data
	POP:    2,  // Simple stack manipulation
	DUP:    3,  // Stack duplication
	SWAP:   3,  // Stack swap operation
	ADD:    3,  // Basic arithmetic
	SUB:    3,  // Basic arithmetic
	MUL:    5,  // More expensive arithmetic
	DIV:    5,  // More expensive arithmetic
	AND:    3,  // Bitwise operation
	OR:     3,  // Bitwise operation
	NOT:    3,  // Bitwise operation
	EQ:     3,  // Comparison
	LT:     3,  // Comparison
	GT:     3,  // Comparison
	ISZERO: 3,  // Comparison
	JUMP:   8,  // Control flow change
	JUMPC:  10, // Conditional control flow
	MSTORE: 20, // Memory write (expensive)
	MLOAD:  15, // Memory read (moderately expensive)
	STOP:   0,  // Program termination (free)
	EMIT:   100, // Debug output (very expensive)
}

func (op OPCODE) Price() uint8 {
	if int(op) < len(opcodeGasPrices) {
		return opcodeGasPrices[op]
	}
	return 255 // Maximum cost for unknown opcodes
}
