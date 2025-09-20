package avm

import (
	"august/types"
	"errors"
)

var (
	ErrOutOfGas                       = errors.New("out of gas")
	ErrStackUnderflow                 = errors.New("stack underflow")
	ErrInvalidInstructionNoRhs        = errors.New("missing rhs side instruction")
	ErrStackOverflow                  = errors.New("stack overflow")
	ErrInvalidJump                    = errors.New("invalid jump destination")
	ErrInvalidOpcode                  = errors.New("invalid opcode")
	ErrMemoryOutOfBounds              = errors.New("memory access out of bounds")
	ErrDivisionByZero                 = errors.New("division by zero")
	ErrExecutionReverted              = errors.New("execution reverted")
	ErrProgramStopped                 = errors.New("program stopped")
	ErrInvalidSwapParameter           = errors.New("invalid swap parameter")
	ErrValueTooLarge                  = errors.New("value exceeds 32-byte limit")
	ErrKeyNotInStorage                = errors.New("key not in storage")

	// Re-export validation errors from types package
	ErrInvalidPush                    = types.ErrInvalidPush
	ErrNegativeValue                  = types.ErrNegativeValue
	ErrUnexpectedInstructionParameter = types.ErrUnexpectedInstructionParameter
)
