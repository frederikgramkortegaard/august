package avm

import (
	"august/config"
	"august/types"
	"errors"
	"fmt"
	"math/big"
)

type Hash32 = types.Hash32

// Signed 256-bit integer helper functions for two's complement arithmetic

var (
	// 2^256 for modulo operations
	mod256 = new(big.Int).Lsh(big.NewInt(1), 256)
	// 2^255 for sign bit check
	sign256 = new(big.Int).Lsh(big.NewInt(1), 255)
)

// toSigned256 converts an unsigned 256-bit big.Int to signed using two's complement interpretation
func toSigned256(n *big.Int) *big.Int {
	// If the sign bit (bit 255) is set, convert to negative
	if n.Cmp(sign256) >= 0 {
		// Calculate two's complement: -(2^256 - n)
		result := new(big.Int).Sub(mod256, n)
		return result.Neg(result)
	}
	// Already positive, return copy
	return new(big.Int).Set(n)
}

// toUnsigned256 converts a signed big.Int to unsigned 256-bit representation using two's complement
func toUnsigned256(n *big.Int) *big.Int {
	if n.Sign() < 0 {
		// Calculate two's complement: 2^256 + n (since n is negative)
		result := new(big.Int).Add(mod256, n)
		return result
	}
	// Positive number, just ensure it's within 256-bit range
	result := new(big.Int).Set(n)
	return result.Mod(result, mod256)
}

// signedAdd performs signed addition with 256-bit overflow handling
func signedAdd(a, b *big.Int) *big.Int {
	// Convert to signed
	signedA := toSigned256(a)
	signedB := toSigned256(b)

	// Perform signed addition
	result := new(big.Int).Add(signedA, signedB)

	// Convert back to unsigned representation
	return toUnsigned256(result)
}

// signedSub performs signed subtraction with 256-bit overflow handling
func signedSub(a, b *big.Int) *big.Int {
	// Convert to signed
	signedA := toSigned256(a)
	signedB := toSigned256(b)

	// Perform signed subtraction
	result := new(big.Int).Sub(signedA, signedB)

	// Convert back to unsigned representation
	return toUnsigned256(result)
}

// signedMul performs signed multiplication with 256-bit overflow handling
func signedMul(a, b *big.Int) *big.Int {
	// Convert to signed
	signedA := toSigned256(a)
	signedB := toSigned256(b)

	// Perform signed multiplication
	result := new(big.Int).Mul(signedA, signedB)

	// Convert back to unsigned representation
	return toUnsigned256(result)
}

// signedDiv performs signed division with proper handling of negative numbers
func signedDiv(a, b *big.Int) *big.Int {
	// Convert to signed
	signedA := toSigned256(a)
	signedB := toSigned256(b)

	// Check for division by zero
	if signedB.Sign() == 0 {
		return big.NewInt(0) // Return 0 for division by zero (could also panic)
	}

	// Perform signed division (truncates toward zero)
	result := new(big.Int).Div(signedA, signedB)

	// Convert back to unsigned representation
	return toUnsigned256(result)
}

// signedCmp compares two unsigned 256-bit values as signed integers
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func signedCmp(a, b *big.Int) int {
	signedA := toSigned256(a)
	signedB := toSigned256(b)
	return signedA.Cmp(signedB)
}

// BlockchainContext contains immutable blockchain data needed for AVM execution
type BlockchainContext struct {
	BlockNumber   uint64          // Current block height
	BlockHash     Hash32          // Current block hash (block being built)
	LastBlockHash Hash32          // Previous block hash (tip/last confirmed)
	Timestamp     uint64          // Current block timestamp
	Difficulty    uint64          // Current block difficulty
	GasLimit      uint64          // Current block gas limit
	Coinbase      types.PublicKey // Current block miner/validator address
	ChainID       uint64          // Chain identifier

	// Transaction context
	Caller          types.PublicKey // Address of message sender (from CallTsx.From)
	Origin          types.PublicKey // Address of transaction originator
	GasPrice        uint64          // Price per gas unit (from CallTsx.GasPrice)
	CallValue       uint64          // Amount of currency sent in this call (from CallTsx.Amount)
	ContractAddress types.PublicKey // Address of the contract being executed
	TsxData         []byte          // Transaction data field

	// Deployment context (for contract calls)
	Deployer       types.PublicKey // Address that originally deployed this contract
	DeploymentTime uint64          // Block timestamp when contract was deployed
	DeploymentGas  uint64          // Gas price used for deployment
}

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

type Runtime struct {
	Stack        *stack
	Memory       *memory
	IC           int // Instruction Counter
	GasAvailable uint64
	GasUsed      uint64
	Instructions []types.Instruction
	Persistent   map[string]string
	Context      *BlockchainContext // Immutable blockchain context data
}

// NewTestRuntime creates a Runtime with minimal blockchain context for testing
func NewTestRuntime(gasAvailable uint64, instructions []types.Instruction) *Runtime {
	return NewRuntime(gasAvailable, instructions, &MockBlockchainContext)
}

// NewTestRuntimeWithLimits creates a Runtime with custom stack and memory limits for testing
func NewTestRuntimeWithLimits(gasAvailable uint64, instructions []types.Instruction, stackSize, memorySize uint64) *Runtime {
	return &Runtime{
		Stack:        NewStack(stackSize),
		Memory:       NewMemory(memorySize),
		IC:           0,
		GasAvailable: gasAvailable,
		GasUsed:      0,
		Instructions: instructions,
		Persistent:   make(map[string]string),
		Context:      &MockBlockchainContext,
	}
}

func NewRuntime(gasAvailable uint64, instructions []types.Instruction, context *BlockchainContext) *Runtime {
	return &Runtime{
		Stack:        NewStack(config.AVMMaxStackSize),
		Memory:       NewMemory(config.AVMMaxMemorySize),
		IC:           0,
		GasAvailable: gasAvailable,
		GasUsed:      0,
		Instructions: instructions,
		Persistent:   make(map[string]string),
		Context:      context,
	}
}

// MockBlockchainContext provides a default blockchain context for testing
var MockBlockchainContext = BlockchainContext{
	BlockNumber:     1,
	BlockHash:       Hash32{},
	LastBlockHash:   Hash32{},
	Timestamp:       1640995200, // Jan 1, 2022
	Difficulty:      1000,
	GasLimit:        1000000,
	Coinbase:        types.PublicKey{}, // Zero address
	ChainID:         1,
	Caller:          types.PublicKey{}, // Zero address
	Origin:          types.PublicKey{}, // Zero address
	GasPrice:        1,
	CallValue:       0,
	ContractAddress: types.PublicKey{}, // Zero address
	Deployer:        types.PublicKey{}, // Zero address
	DeploymentTime:  1640995200,        // Same as genesis block timestamp
	DeploymentGas:   1,
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
		// Convert hex string to big.Int and push
		value, convertErr := ins.GetValueAsBigInt()
		if convertErr != nil {
			err = convertErr
		} else {
			err = r.Stack.Push(value)
		}
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
		// Get swap index from stack
		swapIndexVal, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
		} else {
			swapIndex := int(swapIndexVal.Uint64())
			if swapIndex >= len(r.Stack.s) {
				err = ErrInvalidSwapParameter
			} else {
				err = r.Stack.Swap(swapIndex)
			}
		}
	case ADD:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			// Perform signed addition with two's complement
			res := signedAdd(v2, v1)
			err = r.Stack.Push(res)
		}
	case SUB:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			// Perform signed subtraction with two's complement
			res := signedSub(v2, v1)
			err = r.Stack.Push(res)
		}
	case MUL:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			// Perform signed multiplication with two's complement
			res := signedMul(v2, v1)
			err = r.Stack.Push(res)
		}
	case DIV:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			// Perform signed division with two's complement
			res := signedDiv(v2, v1)
			err = r.Stack.Push(res)
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
			// Perform signed comparison
			if signedCmp(v2, v1) < 0 {
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
			// Perform signed comparison
			if signedCmp(v2, v1) > 0 {
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
		jumpTarget, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
		} else {
			target := int(jumpTarget.Uint64())
			if target < 0 || target >= len(r.Instructions) {
				err = ErrInvalidJump
			} else {
				r.IC = target
			}
		}

	case JUMPC:
		jumpTarget, popErr1 := r.Stack.Pop()
		condition, popErr2 := r.Stack.Pop()
		if popErr1 != nil {
			err = popErr1
		} else if popErr2 != nil {
			err = popErr2
		} else {
			target := int(jumpTarget.Uint64())
			if target < 0 || target >= len(r.Instructions) {
				err = ErrInvalidJump
			} else if condition.Cmp(big.NewInt(1)) == 0 {
				r.IC = target
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
	case PSTORE:
		// PSTORE pops address and value from stack (same as MSTORE pattern)
		address, value, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			// Store value at address in persistent storage
			key := fmt.Sprintf("%d", address.Uint64())
			valueStr := value.String()
			r.Persistent[key] = valueStr
			fmt.Printf("PSTORE: storing key='%s' value='%s'\n", key, valueStr)
		}
	case PLOAD:
		// PLOAD pops address from stack, pushes value to stack (same as MLOAD pattern)
		address, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
		} else {
			key := fmt.Sprintf("%d", address.Uint64())
			valueStr, ok := r.Persistent[key]
			fmt.Printf("PLOAD: looking for key='%s', found=%t", key, ok)
			if ok {
				fmt.Printf(", value='%s'\n", valueStr)
			} else {
				fmt.Printf("\n")
			}

			if !ok {
				// Return 0 if key doesn't exist (like Ethereum)
				err = r.Stack.Push(big.NewInt(0))
			} else {
				// Convert string back to big.Int
				value := new(big.Int)
				value.SetString(valueStr, 10)
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

	// BLOCKCHAIN CONTEXTS
	case CALLER:
		// Push caller address as bytes32
		callerBytes := r.Context.Caller[:]
		callerBig := new(big.Int).SetBytes(callerBytes)
		err = r.Stack.Push(callerBig)
	case ADDRESS:
		// Push contract address as bytes32
		addressBytes := r.Context.ContractAddress[:]
		addressBig := new(big.Int).SetBytes(addressBytes)
		err = r.Stack.Push(addressBig)
	case BALANCE:
		// Pop address from stack, push balance (requires account state - not available in AVM)
		// For now, push 0 as placeholder
		_, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
		} else {
			err = r.Stack.Push(big.NewInt(0))
		}
	case ORIGIN:
		// Push transaction origin address as bytes32
		originBytes := r.Context.Origin[:]
		originBig := new(big.Int).SetBytes(originBytes)
		err = r.Stack.Push(originBig)
	case GASPRICE:
		// Push gas price
		err = r.Stack.Push(big.NewInt(int64(r.Context.GasPrice)))
	case CALLVALUE:
		// Push call value (amount sent in transaction)
		err = r.Stack.Push(big.NewInt(int64(r.Context.CallValue)))
	case TIMESTAMP:
		// Push block timestamp
		err = r.Stack.Push(big.NewInt(int64(r.Context.Timestamp)))
	case DIFFICULTY:
		// Push block difficulty
		err = r.Stack.Push(big.NewInt(int64(r.Context.Difficulty)))
	case BLOCKHASH:
		// Push current block hash as bytes32
		blockHashBytes := r.Context.BlockHash[:]
		blockHashBig := new(big.Int).SetBytes(blockHashBytes)
		err = r.Stack.Push(blockHashBig)
	case LASTBLOCKHASH:
		// Push previous block hash as bytes32
		lastBlockHashBytes := r.Context.LastBlockHash[:]
		lastBlockHashBig := new(big.Int).SetBytes(lastBlockHashBytes)
		err = r.Stack.Push(lastBlockHashBig)
	case COINBASE:
		// Push coinbase address as bytes32
		coinbaseBytes := r.Context.Coinbase[:]
		coinbaseBig := new(big.Int).SetBytes(coinbaseBytes)
		err = r.Stack.Push(coinbaseBig)
	case HEIGHT:
		// Push block height
		err = r.Stack.Push(big.NewInt(int64(r.Context.BlockNumber)))
	case GASLIMIT:
		// Push block gas limit
		err = r.Stack.Push(big.NewInt(int64(r.Context.GasLimit)))
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
