package avm

import "errors"

var (
	ErrOutOfGas                = errors.New("out of gas")
	ErrStackUnderflow          = errors.New("stack underflow")
	ErrInvalidInstructionNoRhs = errors.New("missing rhs side instruction")
	ErrStackOverflow           = errors.New("stack overflow")
	ErrInvalidJump             = errors.New("invalid jump destination")
	ErrInvalidOpcode           = errors.New("invalid opcode")
	ErrMemoryOutOfBounds       = errors.New("memory access out of bounds")
	ErrDivisionByZero          = errors.New("division by zero")
	ErrExecutionReverted       = errors.New("execution reverted")
	ErrInvalidPush             = errors.New("invalid push data")
	ErrProgramStopped          = errors.New("program stopped")
)
