package avm

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
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

	// Blockchain Context
	CALLER     // address of message sender
	ADDRESS    // address of contract currently executing
	BALANCE    // eth balance of an account (takes address from stack pop)
	ORIGIN     // transaction originator
	GASPRICE   // price per gas in this transaction
	CALLVALUE  // amount of AUG sent in this transaction
	TIMESTAMP  // current block timestamp
	DIFFICULTY // current block difficulty
	BLOCKHASH  // block hash (takes block number from stack)
	COINBASE   // current block's beneficiary address
	HEIGHT     // current block number
	GASLIMIT   // current block gas limit

	// COUNTER
	LAST
)

var opcodeNames = [...]string{
	NOOP:       "NOOP",
	PUSH:       "PUSH",
	POP:        "POP",
	DUP:        "DUP",
	SWAP:       "SWAP",
	ADD:        "ADD",
	SUB:        "SUB",
	MUL:        "MUL",
	DIV:        "DIV",
	AND:        "AND",
	OR:         "OR",
	EQ:         "EQ",
	LT:         "LT",
	GT:         "GT",
	ISZERO:     "ISZERO",
	JUMP:       "JUMP",
	JUMPC:      "JUMPC",
	MSTORE:     "MSTORE",
	MLOAD:      "MLOAD",
	PLOAD:      "PLOAD",
	PSTORE:     "PSTORE",
	STOP:       "STOP",
	EMIT:       "EMIT",
	CALLER:     "CALLER",
	ADDRESS:    "ADDRESS",
	BALANCE:    "BALANCE",
	ORIGIN:     "ORIGIN",
	GASPRICE:   "GASPRICE",
	CALLVALUE:  "CALLVALUE",
	TIMESTAMP:  "TIMESTAMP",
	DIFFICULTY: "DIFFICULTY",
	BLOCKHASH:  "BLOCKHASH",
	COINBASE:   "COINBASE",
	HEIGHT:     "HEIGHT",
	GASLIMIT:   "GASLIMIT",
}

var opcodeMap = map[string]OPCODE{
	"NOOP":       NOOP,
	"PUSH":       PUSH,
	"POP":        POP,
	"DUP":        DUP,
	"SWAP":       SWAP,
	"ADD":        ADD,
	"SUB":        SUB,
	"MUL":        MUL,
	"DIV":        DIV,
	"AND":        AND,
	"OR":         OR,
	"EQ":         EQ,
	"LT":         LT,
	"GT":         GT,
	"ISZERO":     ISZERO,
	"JUMP":       JUMP,
	"JUMPC":      JUMPC,
	"MSTORE":     MSTORE,
	"MLOAD":      MLOAD,
	"PLOAD":      PLOAD,
	"PSTORE":     PSTORE,
	"STOP":       STOP,
	"EMIT":       EMIT,
	"CALLER":     CALLER,
	"ADDRESS":    ADDRESS,
	"BALANCE":    BALANCE,
	"ORIGIN":     ORIGIN,
	"GASPRICE":   GASPRICE,
	"CALLVALUE":  CALLVALUE,
	"TIMESTAMP":  TIMESTAMP,
	"DIFFICULTY": DIFFICULTY,
	"BLOCKHASH":  BLOCKHASH,
	"COINBASE":   COINBASE,
	"HEIGHT":     HEIGHT,
	"GASLIMIT":   GASLIMIT,
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
	Opcode OPCODE  `json:"opcode"`
	Value  *string `json:"value,omitempty"` // For PUSH - hex string like "0x2A"
}

// GetValueAsBigInt converts hex Value string to *big.Int
func (ins Instruction) GetValueAsBigInt() (*big.Int, error) {
	if ins.Value == nil || *ins.Value == "" {
		return big.NewInt(0), nil
	}

	// Remove 0x prefix if present
	hexStr := strings.TrimPrefix(*ins.Value, "0x")

	// Parse as hex
	value := new(big.Int)
	_, ok := value.SetString(hexStr, 16)
	if !ok {
		return nil, fmt.Errorf("invalid hex value: %s", *ins.Value)
	}

	return value, nil
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
		if ins.Value == nil || *ins.Value == "" {
			return ErrInvalidPush
		}
		// PUSH values must be valid 32-byte values
		value, err := ins.GetValueAsBigInt()
		if err != nil {
			return err
		}
		if err := validateStackValue(value); err != nil {
			return err
		}
	// All other instructions work with stack values, no parameters needed
	case POP, DUP, SWAP, NOOP, EMIT, STOP,
		 ADD, SUB, MUL, DIV, AND, OR, EQ, LT, GT, ISZERO,
		 JUMP, JUMPC, MSTORE, MLOAD, PC, PSTORE, PLOAD,
		 CALLER, ADDRESS, BALANCE, ORIGIN, GASPRICE, CALLVALUE,
		 TIMESTAMP, DIFFICULTY, BLOCKHASH, COINBASE, HEIGHT, GASLIMIT:
		// These instructions should not have Value
		if ins.Value != nil {
			return ErrUnexpectedInstructionParameter
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

// MakeInstruction creates instructions with no parameters (for all opcodes except PUSH)
func MakeInstruction(opcode OPCODE) Instruction {
	return Instruction{Opcode: opcode, Value: nil}
}

// MakePushInstruction creates a PUSH instruction with a hex value
func MakePushInstruction(value string) Instruction {
	return Instruction{Opcode: PUSH, Value: &value}
}

// instructionJSON is a helper struct for JSON serialization
type instructionJSON struct {
	Opcode string `json:"opcode"`
	Value  string `json:"value,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for Instruction
func (ins Instruction) MarshalJSON() ([]byte, error) {
	result := instructionJSON{
		Opcode: ins.Opcode.String(),
	}

	if ins.Value != nil {
		result.Value = *ins.Value
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

	if temp.Value != "" {
		ins.Value = &temp.Value
	} else {
		ins.Value = nil
	}
	return nil
}
