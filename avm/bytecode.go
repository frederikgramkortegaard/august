package avm

import (
	"encoding/json"
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
	PSTORE
	PLOAD

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
	EQ:     "EQ",
	LT:     "LT",
	GT:     "GT",
	ISZERO: "ISZERO",
	JUMP:   "JUMP",
	JUMPC:  "JUMPC",
	MSTORE: "MSTORE",
	MLOAD:  "MLOAD",
	PLOAD:  "PLOAD",
	PSTORE: "PSTORE",
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
	Opcode OPCODE   `json:"opcode"`
	Value  *big.Int `json:"value,omitempty"` // For PUSH - 32-byte stack values
	Param  uint16   `json:"param,omitempty"` // For SWAP, JUMP, etc. - small integer parameters
}

// ValidateInstruction performs comprehensive validation of instruction parameters
func (ins Instruction) ValidateInstruction() error {
	// Basic opcode validation
	if !ins.Opcode.Valid() {
		return ErrInvalidOpcode
	}

	// Validate fields based on opcode requirements
	switch ins.Opcode {
	case PUSH:
		if ins.Value == nil {
			return ErrInvalidPush
		}
		// PUSH values must be valid 32-byte values
		if err := validateStackValue(ins.Value); err != nil {
			return err
		}
		// PUSH should not have Param
		if ins.Param != 0 {
			return ErrUnexpectedInstructionParameter
		}

	case SWAP:
		// SWAP uses Param field, not Value
		if ins.Value != nil {
			return ErrUnexpectedInstructionParameter
		}

	case POP, DUP, NOOP, EMIT, STOP:
		// These instructions should not have Value or Param
		if ins.Value != nil || ins.Param != 0 {
			return ErrUnexpectedInstructionParameter
		}

	// Arithmetic and comparison operations need values on stack, not instruction parameters
	case ADD, SUB, MUL, DIV, AND, OR, EQ, LT, GT, ISZERO:
		if ins.Value != nil || ins.Param != 0 {
			return ErrUnexpectedInstructionParameter
		}

	// Memory and control flow operations - may use Param for addresses/offsets
	case JUMP, JUMPC:
		// These use Param for jump destinations
		if ins.Value != nil {
			return ErrUnexpectedInstructionParameter
		}
		// Param validation depends on implementation

	case MSTORE, MLOAD, PC, PSTORE, PLOAD:
		// These may use either Value or Param depending on implementation
		// For now, allow both but not together
		if ins.Value != nil && ins.Param != 0 {
			return ErrUnexpectedInstructionParameter
		}
		if ins.Value != nil {
			if err := validateStackValue(ins.Value); err != nil {
				return err
			}
		}

	default:
		return ErrInvalidOpcode
	}

	return nil
}

// validateStackValue ensures a big.Int represents a valid 32-byte stack value
func validateStackValue(val *big.Int) error {
	if val == nil {
		return ErrInvalidPush
	}

	// No negative values allowed
	if val.Sign() < 0 {
		return ErrNegativeValue
	}

	// Must fit in 32 bytes (256 bits)
	if val.BitLen() > 256 {
		return ErrValueTooLarge
	}

	return nil
}

// Helper functions for cleaner instruction creation

// MakeInstruction creates instructions with no parameters
func MakeInstruction(opcode OPCODE) Instruction {
	return Instruction{Opcode: opcode, Value: nil, Param: 0}
}

// MakeInstructionWithValue creates instructions with a value
func MakeInstructionWithValue(opcode OPCODE, value *big.Int) Instruction {
	return Instruction{Opcode: opcode, Value: value, Param: 0}
}

// MakeInstructionWithParam creates instructions with a parameter
func MakeInstructionWithParam(opcode OPCODE, param uint16) Instruction {
	return Instruction{Opcode: opcode, Value: nil, Param: param}
}

// MakeInstructionWithValueAndParam creates instructions with both value and parameter
func MakeInstructionWithValueAndParam(opcode OPCODE, value *big.Int, param uint16) Instruction {
	return Instruction{Opcode: opcode, Value: value, Param: param}
}

// instructionJSON is a helper struct for JSON serialization
type instructionJSON struct {
	Opcode string `json:"opcode"`
	Value  string `json:"value,omitempty"`
	Param  uint16 `json:"param,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for Instruction
func (ins Instruction) MarshalJSON() ([]byte, error) {
	result := instructionJSON{
		Opcode: ins.Opcode.String(),
		Param:  ins.Param,
	}

	// Handle big.Int serialization as hex string
	if ins.Value != nil {
		result.Value = "0x" + ins.Value.Text(16)
	}

	return json.Marshal(result)
}

// UnmarshalJSON implements custom JSON unmarshaling for Instruction
func (ins *Instruction) UnmarshalJSON(data []byte) error {
	var temp instructionJSON
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Parse opcode from string
	for op := OPCODE(0); op <= OPCODE(255); op++ {
		if op.String() == temp.Opcode {
			ins.Opcode = op
			break
		}
	}

	// Parse big.Int from hex string
	if temp.Value != "" {
		value := new(big.Int)
		if temp.Value[:2] == "0x" {
			value.SetString(temp.Value[2:], 16)
		} else {
			value.SetString(temp.Value, 16)
		}
		ins.Value = value
	}

	ins.Param = temp.Param
	return nil
}
