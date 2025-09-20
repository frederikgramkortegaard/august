package avm

import "august/types"

type OPCODE = types.OPCODE
type Instruction = types.Instruction

const (
	NOOP          = types.NOOP
	PUSH          = types.PUSH
	POP           = types.POP
	DUP           = types.DUP
	SWAP          = types.SWAP
	ADD           = types.ADD
	SUB           = types.SUB
	MUL           = types.MUL
	DIV           = types.DIV
	AND           = types.AND
	OR            = types.OR
	EQ            = types.EQ
	LT            = types.LT
	GT            = types.GT
	ISZERO        = types.ISZERO
	JUMP          = types.JUMP
	JUMPC         = types.JUMPC
	MSTORE        = types.MSTORE
	MLOAD         = types.MLOAD
	PSTORE        = types.PSTORE
	PLOAD         = types.PLOAD
	STOP          = types.STOP
	EMIT          = types.EMIT
	CALLER        = types.CALLER
	ADDRESS       = types.ADDRESS
	BALANCE       = types.BALANCE
	ORIGIN        = types.ORIGIN
	GASPRICE      = types.GASPRICE
	CALLVALUE     = types.CALLVALUE
	TIMESTAMP     = types.TIMESTAMP
	DIFFICULTY    = types.DIFFICULTY
	BLOCKHASH     = types.BLOCKHASH
	LASTBLOCKHASH = types.LASTBLOCKHASH
	COINBASE      = types.COINBASE
	HEIGHT        = types.HEIGHT
	GASLIMIT      = types.GASLIMIT
	LAST          = types.LAST
)

var MakeInstruction = types.MakeInstruction
var MakePushInstruction = types.MakePushInstruction
var MakeInstructionWithValue = types.MakeInstructionWithValue
var MakeInstructionWithValueAndParam = types.MakeInstructionWithValueAndParam
var MakeInstructionWithParam = types.MakeInstructionWithParam
var opcodeMap = types.OpcodeMap