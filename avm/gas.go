package avm

var opcodeGasPrices = [...]uint16{
	NOOP:       1,
	PUSH:       3,
	POP:        2,
	DUP:        3,
	SWAP:       3,
	ADD:        3,
	SUB:        3,
	MUL:        5,
	DIV:        5,
	AND:        3,
	OR:         3,
	EQ:         3,
	LT:         3,
	GT:         3,
	ISZERO:     3,
	JUMP:       8,
	JUMPC:      10,
	MSTORE:     20,
	MLOAD:      15,
	PSTORE:     1000,
	PLOAD:      1000,
	STOP:       0,
	EMIT:       100,
	CALLER:     2,
	ADDRESS:    2,
	BALANCE:    400,
	ORIGIN:     2,
	GASPRICE:   2,
	CALLVALUE:  2,
	TIMESTAMP:  2,
	DIFFICULTY: 2,
	BLOCKHASH:  20,
	COINBASE:   2,
	HEIGHT:     2,
	GASLIMIT:   2,
}

func (op OPCODE) Price() uint16 {
	if int(op) < len(opcodeGasPrices) {
		return opcodeGasPrices[op]
	}
	return 65535 // Maximum cost for unknown opcodes
}
