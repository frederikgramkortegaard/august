package avm

import (
	"errors"
	"fmt"
	"math/big"
)

type stack struct {
	s []*big.Int
}

func NewStack() *stack {
	return &stack{make([]*big.Int, 0)}
}

func (s *stack) Push(v *big.Int) error {
	s.s = append(s.s, v)
	return nil
}

func (s *stack) Pop() (*big.Int, error) {
	l := len(s.s)
	if l == 0 {
		return nil, errors.New("Empty Stack")
	}

	res := s.s[l-1]
	s.s = s.s[:l-1]
	return res, nil
}

func (s *stack) Top() (*big.Int, error) {
	l := len(s.s)
	if l == 0 {
		return nil, errors.New("Empty Stack")
	}
	return s.s[l-1], nil
}

// stack has len() = 10, swap0 would do nothing, swap10 would require len=11

func (s *stack) Swap(down int) error {
	l := len(s.s)
	if l < down {
		return ErrStackUnderflow
	}

	// Swap top element with element 'down' positions from top
	s.s[l-1], s.s[l-down] = s.s[l-down], s.s[l-1]
	return nil
}

type Runtime struct {
	Stack        *stack
	IC           uint64 // Instruction Counter
	GasAvailable uint64
	GasUsed      uint64
}

func NewRuntime(gasAvailable uint64) *Runtime {
	return &Runtime{
		Stack:        NewStack(),
		IC:           0,
		GasAvailable: gasAvailable,
		GasUsed:      0,
	}
}

func (r *Runtime) StartExecution(instructions []Instruction) (uint64, error) {
	// Validate that the stack is clean
	if r.IC != 0 {
		return 0, errors.New("Instruction counter is not zero, runtime is not clean")
	}

	if len(r.Stack.s) != 0 {
		return 0, errors.New("Stack is not empty, runtime is not clean")
	}

	// Execute Instructions
	for _, ins := range instructions {
		err := r.ExecuteInstruction(ins)
		if err != nil {
			return r.GasUsed, err
		}
	}

	return r.GasUsed, nil
}

func (r *Runtime) ExecuteInstruction(ins Instruction) error {
	oc := ins.Opcode

	if !oc.Valid() {
		return ErrInvalidOpcode
	}

	if r.GasAvailable < uint64(oc.Price()) {
		return ErrOutOfGas
	}

	r.IC++
	err := r.Stack.ExecuteInstruction(ins)

	r.GasUsed += uint64(oc.Price())
	r.GasAvailable -= uint64(oc.Price())

	return err
}

func (s *stack) ExecuteInstruction(ins Instruction) error {
	switch ins.Opcode {
	case NOOP:
		// Nothing
	case PUSH:
		if ins.Rhs == nil {
			return ErrInvalidPush
		}
		// Ensure it's a 32-byte value
		bytes := ins.Rhs.Bytes()
		if len(bytes) > 32 {
			return ErrInvalidPush
		}
		return s.Push(ins.Rhs)
	case POP:
		_, err := s.Pop()
		return err
	case DUP:
		val, err := s.Top()
		if err != nil {
			return err
		}
		return s.Push(val)
	case SWAP:
		if ins.Rhs == nil {
			return ErrInvalidInstructionNoRhs
		}
		if !ins.Rhs.IsInt64() {
			return errors.New("swap distance too large")
		}
		return s.Swap(int(ins.Rhs.Int64()))

	case EMIT:
		val, err := s.Top()
		if err != nil {
			return err
		}
		var b [32]byte
		val.FillBytes(b[:])
		fmt.Printf("AVM EMIT: %x\n", b)
	default:
		return ErrInvalidOpcode
	}
	return nil
}
