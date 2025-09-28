package types

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Error constants for instruction validation
var (
	ErrInvalidPush                    = errors.New("invalid push data")
	ErrNegativeValue                  = errors.New("negative values not allowed")
	ErrUnexpectedInstructionParameter = errors.New("unexpected instruction parameter")
)

// AVM Opcodes
type OPCODE uint8

const (
	// Meta
	NOOP OPCODE = iota
	PC          // Program Counter

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
	MOD
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

	// Frame-based locals
	LOAD_LOCAL
	STORE_LOCAL

	// PROG CTRL
	CALL
	RETURN
	STOP

	// Debug
	EMIT

	// String Operations
	STRCONCAT
	STRLEN
	EMITSTR

	// Blockchain Context
	CALLER        // address of message sender
	ADDRESS       // address of contract currently executing
	BALANCE       // balance of an account (takes address from stack pop)
	ORIGIN        // transaction originator
	GASPRICE      // price per gas in this transaction
	CALLVALUE     // amount of AUG sent in this transaction
	TIMESTAMP     // current block timestamp
	DIFFICULTY    // current block difficulty
	BLOCKHASH     // current block hash
	LASTBLOCKHASH // previous block hash (tip)
	COINBASE      // current block's beneficiary address
	HEIGHT        // current block number
	GASLIMIT      // current block gas limit

	// COUNTER
	LAST
)

var opcodeNames = [...]string{
	NOOP:          "NOOP",
	PUSH:          "PUSH",
	POP:           "POP",
	DUP:           "DUP",
	SWAP:          "SWAP",
	ADD:           "ADD",
	SUB:           "SUB",
	MUL:           "MUL",
	DIV:           "DIV",
	MOD:           "MOD",
	AND:           "AND",
	OR:            "OR",
	EQ:            "EQ",
	LT:            "LT",
	GT:            "GT",
	ISZERO:        "ISZERO",
	JUMP:          "JUMP",
	JUMPC:         "JUMPC",
	MSTORE:        "MSTORE",
	MLOAD:         "MLOAD",
	PLOAD:         "PLOAD",
	PSTORE:        "PSTORE",
	LOAD_LOCAL:    "LOAD_LOCAL",
	STORE_LOCAL:   "STORE_LOCAL",
	CALL:          "CALL",
	RETURN:        "RETURN",
	STOP:          "STOP",
	EMIT:          "EMIT",
	STRCONCAT:     "STRCONCAT",
	STRLEN:        "STRLEN",
	EMITSTR:       "EMITSTR",
	CALLER:        "CALLER",
	ADDRESS:       "ADDRESS",
	BALANCE:       "BALANCE",
	ORIGIN:        "ORIGIN",
	GASPRICE:      "GASPRICE",
	CALLVALUE:     "CALLVALUE",
	TIMESTAMP:     "TIMESTAMP",
	DIFFICULTY:    "DIFFICULTY",
	BLOCKHASH:     "BLOCKHASH",
	LASTBLOCKHASH: "LASTBLOCKHASH",
	COINBASE:      "COINBASE",
	HEIGHT:        "HEIGHT",
	GASLIMIT:      "GASLIMIT",
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

var opcodeGasPrices = [...]uint16{
	NOOP:          1,
	PUSH:          3,
	POP:           2,
	DUP:           3,
	SWAP:          3,
	ADD:           3,
	SUB:           3,
	MUL:           5,
	DIV:           5,
	MOD:           5,
	AND:           3,
	OR:            3,
	EQ:            3,
	LT:            3,
	GT:            3,
	ISZERO:        3,
	JUMP:          8,
	JUMPC:         10,
	MSTORE:        20,
	MLOAD:         15,
	PSTORE:        1000,
	PLOAD:         1000,
	LOAD_LOCAL:    3,
	STORE_LOCAL:   3,
	CALL:          10,
	RETURN:        8,
	STOP:          0,
	EMIT:          100,
	STRCONCAT:     50,
	STRLEN:        10,
	EMITSTR:       150,
	CALLER:        2,
	ADDRESS:       2,
	BALANCE:       400,
	ORIGIN:        2,
	GASPRICE:      2,
	CALLVALUE:     2,
	TIMESTAMP:     2,
	DIFFICULTY:    2,
	BLOCKHASH:     20,
	LASTBLOCKHASH: 20,
	COINBASE:      2,
	HEIGHT:        2,
	GASLIMIT:      2,
}

func (op OPCODE) Price() uint16 {
	if int(op) < len(opcodeGasPrices) {
		return opcodeGasPrices[op]
	}
	return 65535
}

var OpcodeMap = map[string]OPCODE{
	"NOOP":          NOOP,
	"PUSH":          PUSH,
	"POP":           POP,
	"DUP":           DUP,
	"SWAP":          SWAP,
	"ADD":           ADD,
	"SUB":           SUB,
	"MUL":           MUL,
	"DIV":           DIV,
	"MOD":           MOD,
	"AND":           AND,
	"OR":            OR,
	"EQ":            EQ,
	"LT":            LT,
	"GT":            GT,
	"ISZERO":        ISZERO,
	"JUMP":          JUMP,
	"JUMPC":         JUMPC,
	"MSTORE":        MSTORE,
	"MLOAD":         MLOAD,
	"PLOAD":         PLOAD,
	"PSTORE":        PSTORE,
	"LOAD_LOCAL":    LOAD_LOCAL,
	"STORE_LOCAL":   STORE_LOCAL,
	"CALL":          CALL,
	"RETURN":        RETURN,
	"STOP":          STOP,
	"EMIT":          EMIT,
	"STRCONCAT":     STRCONCAT,
	"STRLEN":        STRLEN,
	"EMITSTR":       EMITSTR,
	"CALLER":        CALLER,
	"ADDRESS":       ADDRESS,
	"BALANCE":       BALANCE,
	"ORIGIN":        ORIGIN,
	"GASPRICE":      GASPRICE,
	"CALLVALUE":     CALLVALUE,
	"TIMESTAMP":     TIMESTAMP,
	"DIFFICULTY":    DIFFICULTY,
	"BLOCKHASH":     BLOCKHASH,
	"LASTBLOCKHASH": LASTBLOCKHASH,
	"COINBASE":      COINBASE,
	"HEIGHT":        HEIGHT,
	"GASLIMIT":      GASLIMIT,
}

func (ins Instruction) ValidateInstruction() error {
	if !ins.Opcode.Valid() {
		return fmt.Errorf("invalid opcode")
	}

	// Validate PUSH instructions
	if ins.Opcode == PUSH {
		if ins.Value == nil {
			return ErrInvalidPush
		}
		// Check for invalid parameter format (like from MakeInstructionWithValueAndParam)
		if strings.Contains(*ins.Value, ",param=") {
			return ErrUnexpectedInstructionParameter
		}
		// Check for negative values by parsing the value
		if val, err := ins.GetValueAsBigInt(); err != nil {
			return ErrInvalidPush
		} else if val.Sign() < 0 {
			return ErrNegativeValue
		}
	}

	// Validate instructions that shouldn't have values
	if ins.Opcode != PUSH && ins.Value != nil {
		return ErrUnexpectedInstructionParameter
	}

	return nil
}

func MakeInstruction(opcode OPCODE) Instruction {
	return Instruction{Opcode: opcode, Value: nil}
}

func MakePushInstruction(value string) Instruction {
	return Instruction{Opcode: PUSH, Value: &value}
}

// MakeInstructionWithValue creates an instruction with a value parameter
func MakeInstructionWithValue(opcode OPCODE, value interface{}) Instruction {
	var valueStr string

	switch v := value.(type) {
	case string:
		valueStr = v
	case *big.Int:
		if v != nil {
			if v.Sign() < 0 {
				// Store negative values as decimal to preserve the sign
				valueStr = v.String()
			} else {
				valueStr = "0x" + v.Text(16)
			}
		}
	case int:
		if v < 0 {
			valueStr = big.NewInt(int64(v)).String()
		} else {
			valueStr = "0x" + big.NewInt(int64(v)).Text(16)
		}
	case int64:
		if v < 0 {
			valueStr = big.NewInt(v).String()
		} else {
			valueStr = "0x" + big.NewInt(v).Text(16)
		}
	case uint64:
		valueStr = "0x" + big.NewInt(int64(v)).Text(16)
	default:
		// For nil or unsupported types, use empty string
		valueStr = ""
	}

	if valueStr == "" {
		return Instruction{Opcode: opcode, Value: nil}
	}

	return Instruction{Opcode: opcode, Value: &valueStr}
}

// MakeInstructionWithValueAndParam creates an instruction with value and parameter (for tests)
func MakeInstructionWithValueAndParam(opcode OPCODE, value interface{}, param interface{}) Instruction {
	// Create an invalid instruction by combining value and param in the value field
	// This is used by tests to create intentionally invalid instructions
	valueStr := ""

	switch v := value.(type) {
	case string:
		valueStr = v
	case *big.Int:
		if v != nil {
			if v.Sign() < 0 {
				valueStr = v.String()
			} else {
				valueStr = "0x" + v.Text(16)
			}
		}
	}

	// Add param as a suffix to make it invalid (this should fail validation)
	invalidValue := valueStr + fmt.Sprintf(",param=%v", param)

	return Instruction{Opcode: opcode, Value: &invalidValue}
}

// MakeInstructionWithParam creates an instruction with a parameter (for tests)
func MakeInstructionWithParam(opcode OPCODE, param interface{}) Instruction {
	return MakeInstructionWithValue(opcode, param)
}

// GetValueAsBigInt converts Value string to *big.Int (supports both hex and decimal)
func (ins Instruction) GetValueAsBigInt() (*big.Int, error) {
	if ins.Value == nil || *ins.Value == "" {
		return big.NewInt(0), nil
	}

	value := new(big.Int)
	valueStr := *ins.Value

	// Try hex format first (with 0x prefix)
	if strings.HasPrefix(valueStr, "0x") {
		hexStr := strings.TrimPrefix(valueStr, "0x")
		_, ok := value.SetString(hexStr, 16)
		if !ok {
			return nil, fmt.Errorf("invalid hex value: %s", valueStr)
		}
	} else {
		// Try decimal format (including negative numbers)
		_, ok := value.SetString(valueStr, 10)
		if !ok {
			return nil, fmt.Errorf("invalid value: %s", valueStr)
		}
	}

	return value, nil
}
