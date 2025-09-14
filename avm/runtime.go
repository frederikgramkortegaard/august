package avm

import (
	"errors"
	"fmt"
	"math/big"
)

type stack struct {
	s       []*big.Int
	maxSize uint64
}

func NewStack(maxSize uint64) *stack {
	return &stack{
		s:       make([]*big.Int, 0),
		maxSize: maxSize,
	}
}

func (s *stack) Push(v *big.Int) error {
	if s.maxSize > 0 && uint64(len(s.s)) >= s.maxSize {
		return ErrStackOverflow
	}
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

func (s *stack) Swap(down int) error {
	if len(s.s) <= down {
		return ErrStackUnderflow
	}

	cpy, err := s.Top()
	if err != nil {
		return err
	}

	l := len(s.s)
	s.s[l-1], s.s[l-1-down] = s.s[l-1-down], cpy
	return nil
}
func (s *stack) Pop2() (*big.Int, *big.Int, error) {
	v1, err := s.Pop()
	if err != nil {
		return nil, nil, err
	}
	v2, err := s.Pop()
	if err != nil {
		return nil, nil, err
	}
	return v1, v2, nil
}

func (s *stack) Size() int {
	return len(s.s)
}

type memory struct {
	m       map[uint64]*big.Int
	maxSize uint64
}

func NewMemory(maxSize uint64) *memory {
	return &memory{
		m:       make(map[uint64]*big.Int),
		maxSize: maxSize,
	}
}

func (m *memory) Store(address uint64, value *big.Int) error {
	if m.maxSize > 0 && address >= m.maxSize {
		return ErrMemoryOutOfBounds
	}
	m.m[address] = new(big.Int).Set(value)
	return nil
}

func (m *memory) Load(address uint64) (*big.Int, error) {
	if m.maxSize > 0 && address >= m.maxSize {
		return nil, ErrMemoryOutOfBounds
	}
	if value, exists := m.m[address]; exists {
		return new(big.Int).Set(value), nil
	}
	return big.NewInt(0), nil
}

type RuntimeConfig struct {
	StackSize  uint64 // [32]byte * StackSize
	MemorySize uint64 // [32]byte * MemorySize

}

type Runtime struct {
	Config       RuntimeConfig
	Stack        *stack
	Memory       *memory
	IC           int // Instruction Counter
	GasAvailable uint64
	GasUsed      uint64
	Instructions []Instruction
}

func NewRuntime(gasAvailable uint64, instructions []Instruction, config RuntimeConfig) *Runtime {
	return &Runtime{
		Config:       config,
		Stack:        NewStack(config.StackSize),
		Memory:       NewMemory(config.MemorySize),
		IC:           0,
		GasAvailable: gasAvailable,
		GasUsed:      0,
		Instructions: instructions,
	}
}

func (r *Runtime) StartExecution() (uint64, error) {
	// Validate that the stack is clean
	if r.IC != 0 {
		return 0, errors.New("Instruction counter is not zero, runtime is not clean")
	}

	if len(r.Stack.s) != 0 {
		return 0, errors.New("Stack is not empty, runtime is not clean")
	}

	// Execute Instructions
	for r.IC < len(r.Instructions) {
		err := r.ExecuteInstruction()
		if err != nil {
			return r.GasUsed, err
		}
	}

	return r.GasUsed, nil
}

func (r *Runtime) ExecuteInstruction() error {
	ins := r.Instructions[r.IC]
	// Validate instruction before execution
	if err := ins.ValidateInstruction(); err != nil {
		return err
	}

	oc := ins.Opcode

	if r.GasAvailable < uint64(oc.Price()) {
		return ErrOutOfGas
	}

	// Execute the instruction
	var err error
	switch ins.Opcode {
	case NOOP:
		// Nothing
	case PUSH:
		// Validation already done, safe to push
		err = r.Stack.Push(ins.Value)
	case POP:
		_, err = r.Stack.Pop()
	case DUP:
		val, popErr := r.Stack.Top()
		if popErr != nil {
			err = popErr
		} else {
			err = r.Stack.Push(val)
		}
	case SWAP:
		// Validation already done, safe to convert from uint16
		err = r.Stack.Swap(int(ins.Param))
	case ADD:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			res := new(big.Int).Add(v2, v1)
			mod := new(big.Int).Lsh(big.NewInt(1), 256) // 2^256
			err = r.Stack.Push(res.Mod(res, mod))
		}
	case SUB:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			res := new(big.Int).Sub(v2, v1)
			mod := new(big.Int).Lsh(big.NewInt(1), 256) // 2^256
			err = r.Stack.Push(res.Mod(res, mod))
		}
	case MUL:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			res := new(big.Int).Mul(v2, v1)
			mod := new(big.Int).Lsh(big.NewInt(1), 256) // 2^256
			err = r.Stack.Push(res.Mod(res, mod))
		}
	case DIV:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			res := new(big.Int).Div(v2, v1)
			mod := new(big.Int).Lsh(big.NewInt(1), 256) // 2^256
			err = r.Stack.Push(res.Mod(res, mod))
		}
	case AND:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			res := new(big.Int).And(v2, v1)
			mod := new(big.Int).Lsh(big.NewInt(1), 256) // 2^256
			err = r.Stack.Push(res.Mod(res, mod))
		}
	case OR:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			res := new(big.Int).Or(v2, v1)
			mod := new(big.Int).Lsh(big.NewInt(1), 256) // 2^256
			err = r.Stack.Push(res.Mod(res, mod))
		}
	case EQ:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			if v2.Cmp(v1) == 0 {
				err = r.Stack.Push(big.NewInt(1))
			} else {
				err = r.Stack.Push(big.NewInt(0))
			}
		}
	case LT:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			if v2.Cmp(v1) < 0 {
				err = r.Stack.Push(big.NewInt(1))
			} else {
				err = r.Stack.Push(big.NewInt(0))
			}
		}
	case GT:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			if v2.Cmp(v1) > 0 {
				err = r.Stack.Push(big.NewInt(1))
			} else {
				err = r.Stack.Push(big.NewInt(0))
			}
		}
	case ISZERO:
		val, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
		} else {
			if val.Cmp(big.NewInt(0)) == 0 {
				err = r.Stack.Push(big.NewInt(1))
			} else {
				err = r.Stack.Push(big.NewInt(0))
			}
		}
	case JUMP:
		if int(ins.Param) < 0 || int(ins.Param) >= len(r.Instructions) {
			return ErrInvalidJump
		}
		r.IC = int(ins.Param)

	case JUMPC:
		if int(ins.Param) < 0 || int(ins.Param) >= len(r.Instructions) {
			return ErrInvalidJump
		}
		val, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
		} else {
			if val.Cmp(big.NewInt(1)) == 0 {
				r.IC = int(ins.Param)
			} else {
				r.IC++ // Continue to next instruction if condition is false
			}
		}

	case MSTORE:
		value, address, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			err = r.Memory.Store(address.Uint64(), value)
		}
	case MLOAD:
		address, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
		} else {
			value, loadErr := r.Memory.Load(address.Uint64())
			if loadErr != nil {
				err = loadErr
			} else {
				err = r.Stack.Push(value)
			}
		}
	case STOP:
		return ErrProgramStopped

	case EMIT:
		val, popErr := r.Stack.Top()
		if popErr != nil {
			err = popErr
		} else {
			var b [32]byte
			val.FillBytes(b[:])
			fmt.Printf("AVM EMIT: %x\n", b)
		}
	default:
		err = ErrInvalidOpcode
	}

	r.GasUsed += uint64(oc.Price())
	r.GasAvailable -= uint64(oc.Price())

	// Increment instruction counter for non-jump instructions
	if ins.Opcode != JUMP && ins.Opcode != JUMPC {
		r.IC++
	}

	return err
}
