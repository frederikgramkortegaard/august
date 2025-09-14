package avm

import (
	"fmt"
	"math/big"
)

// AVM Opcodes
type OPCODE uint8

const (

	// Meta
	NOOP OPCODE = iota
	PC          // Program Coutner

	// Stack Manipulation
	PUSH
	POP
	DUP
	SWAP

	// Arithmetic
	ADD
	SUB
	MUL
	DIV
	AND
	OR
	NOT

	// Conditionals
	EQ
	LT
	GT
	ISZERO
	JUMP
	JUMPC // Jump Conditional (if 1 is the last on stack)

	// Memory
	MSTORE
	MLOAD

	// PROG CTRL
	STOP

	// Debug
	EMIT

	// COUNTER
	LAST
)

var opcodeNames = [...]string{
	NOOP:   "NOOP",
	PUSH:   "PUSH",
	POP:    "POP",
	DUP:    "DUP",
	SWAP:   "SWAP",
	ADD:    "ADD",
	SUB:    "SUB",
	MUL:    "MUL",
	DIV:    "DIV",
	AND:    "AND",
	OR:     "OR",
	NOT:    "NOT",
	EQ:     "EQ",
	LT:     "LT",
	GT:     "GT",
	ISZERO: "ISZERO",
	JUMP:   "JUMP",
	JUMPC:  "JUMPC",
	MSTORE: "MSTORE",
	MLOAD:  "MLOAD",
	STOP:   "STOP",
	EMIT:   "EMIT",
}

func (op OPCODE) Valid() bool {
	return op < LAST && op >= 0
}

func (op OPCODE) String() string {
	if int(op) < len(opcodeNames) && opcodeNames[op] != "" {
		return opcodeNames[op]
	}
	return fmt.Sprintf("UNKNOWN_OPCODE(%d)", op)
}

type Instruction struct {
	Opcode OPCODE
	Rhs    *big.Int
}
