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
	Stack          *stack
	Memory         *memory
	FramePointer   int // Points to base of current call frame on stack
	IC             int // Instruction Counter
	StartIC        int // Initial instruction counter for validation
	GasAvailable   uint64
	GasUsed        uint64
	Instructions   []types.Instruction
	Persistent     map[string]string
	Context        *BlockchainContext // Immutable blockchain context data
	ReturnStack    []int // Stack of return addresses for function calls
	FrameStack     []int // Stack of frame pointers for function calls
	NextMemoryAddr uint64 // Next available memory address for dynamic allocation
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
	return NewRuntimeWithIC(gasAvailable, instructions, context, 0)
}

func NewRuntimeWithIC(gasAvailable uint64, instructions []types.Instruction, context *BlockchainContext, startIC int) *Runtime {
	return &Runtime{
		Stack:          NewStack(config.AVMMaxStackSize),
		Memory:         NewMemory(config.AVMMaxMemorySize),
		FramePointer:   0,
		IC:             startIC,
		StartIC:        startIC,
		GasAvailable:   gasAvailable,
		GasUsed:        0,
		Instructions:   instructions,
		Persistent:     make(map[string]string),
		Context:        context,
		ReturnStack:    make([]int, 0),
		FrameStack:     make([]int, 0),
		NextMemoryAddr: 0,
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
	// Validate that the runtime is clean
	if r.IC != r.StartIC {
		return 0, fmt.Errorf("Instruction counter is %d, expected %d, runtime is not clean", r.IC, r.StartIC)
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
	case MOD:
		v1, v2, popErr := r.Stack.Pop2()
		if popErr != nil {
			err = popErr
		} else {
			// Modulo operation
			if v1.Sign() == 0 {
				err = r.Stack.Push(big.NewInt(0))
			} else {
				res := new(big.Int).Mod(v2, v1)
				err = r.Stack.Push(res)
			}
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
		address, value, popErr := r.Stack.Pop2()
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
			// Check if address points to a string in memory
			var key string
			addressUint := address.Uint64()

			// Try to read string from memory at this address
			if stringContent := r.readStringFromMemory(addressUint); stringContent != "" {
				// Use actual string content as key
				key = stringContent
			} else {
				// Fallback to numeric key for non-string addresses
				key = fmt.Sprintf("%d", addressUint)
			}

			// Check if value points to a string in memory
			var valueStr string
			valueUint := value.Uint64()
			// Try to read string from memory at this address
			if stringContent := r.readStringFromMemory(valueUint); stringContent != "" {
				// Store actual string content, not memory address
				valueStr = stringContent
			} else {
				// Fallback to numeric value for non-string values
				valueStr = value.String()
			}
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
				// Return empty string address if key doesn't exist
				emptyStringAddr := r.storeStringInMemory("")
				err = r.Stack.Push(big.NewInt(int64(emptyStringAddr)))
			} else {
				// Store the retrieved string in memory and return its address
				stringAddr := r.storeStringInMemory(valueStr)
				err = r.Stack.Push(big.NewInt(int64(stringAddr)))
			}
		}
	case LOAD_LOCAL:
		offsetBig, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
		} else {
			offset := int(offsetBig.Uint64())
			stackIndex := r.FramePointer + offset
			if stackIndex < 0 {
				err = errors.New("LOAD_LOCAL: invalid offset")
			} else {
				// Expand stack if necessary (like STORE_LOCAL)
				for len(r.Stack.s) <= stackIndex {
					r.Stack.s = append(r.Stack.s, big.NewInt(0))
				}
				value := r.Stack.s[stackIndex]
				err = r.Stack.Push(value)
			}
		}
	case STORE_LOCAL:
		offsetBig, popErr1 := r.Stack.Pop()
		value, popErr2 := r.Stack.Pop()
		if popErr1 != nil {
			err = popErr1
		} else if popErr2 != nil {
			err = popErr2
		} else {
			offset := int(offsetBig.Uint64())
			stackIndex := r.FramePointer + offset
			if stackIndex < 0 {
				err = errors.New("STORE_LOCAL: invalid offset")
			} else {
				// Expand stack if necessary
				for len(r.Stack.s) <= stackIndex {
					r.Stack.s = append(r.Stack.s, big.NewInt(0))
				}
				r.Stack.s[stackIndex] = value
			}
		}
	case CALL:
		// Pop function address from stack
		functionAddr, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
			break
		}

		// Save current frame pointer
		r.FrameStack = append(r.FrameStack, r.FramePointer)

		// Set new frame pointer to point to the function arguments
		// After popping function address, the arguments are on top of stack
		// For add(a, b), stack now has: [..., arg0, arg1]
		// We want FramePointer to point to arg0
		numParams := 2 // TODO: get actual parameter count from function metadata
		r.FramePointer = len(r.Stack.s) - numParams

		// Push return address onto return stack
		returnAddr := r.IC + 1 // Next instruction after CALL
		r.ReturnStack = append(r.ReturnStack, returnAddr)

		// Jump to function address
		r.IC = int(functionAddr.Uint64()) - 1 // -1 because IC will be incremented after
	case RETURN:
		// Pop return address from return stack and jump back
		if len(r.ReturnStack) == 0 {
			// No return address - this is a top-level function return, stop execution
			return ErrProgramStopped
		}

		// The return value should be on top of the stack
		// We need to preserve it across the frame restoration
		returnValue, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
			break
		}

		// Restore frame pointer
		if len(r.FrameStack) > 0 {
			r.FramePointer = r.FrameStack[len(r.FrameStack)-1]
			r.FrameStack = r.FrameStack[:len(r.FrameStack)-1]
		}

		// Clear function parameters from stack (restore to pre-call state)
		// For add(a, b), we need to pop the 2 parameters that were pushed by caller
		for i := 0; i < 2; i++ { // TODO: get actual parameter count
			if len(r.Stack.s) > 0 {
				r.Stack.Pop()
			}
		}

		// Push return value back onto stack for caller
		err = r.Stack.Push(returnValue)
		if err != nil {
			break
		}

		// Pop return address and jump back
		returnAddr := r.ReturnStack[len(r.ReturnStack)-1]
		r.ReturnStack = r.ReturnStack[:len(r.ReturnStack)-1]
		r.IC = returnAddr - 1 // -1 because IC will be incremented after
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

	case types.STRCONCAT:
		ptr2, err2 := r.Stack.Pop()
		if err2 != nil {
			err = err2
			break
		}
		ptr1, err1 := r.Stack.Pop()
		if err1 != nil {
			err = err1
			break
		}

		addr1 := ptr1.Uint64()
		addr2 := ptr2.Uint64()

		len1Val, err := r.Memory.Load(addr1)
		if err != nil {
			err = errors.New("invalid string pointer 1")
			break
		}
		len1 := len1Val.Uint64()

		len2Val, err := r.Memory.Load(addr2)
		if err != nil {
			err = errors.New("invalid string pointer 2")
			break
		}
		len2 := len2Val.Uint64()

		chunks1 := (len1 + 31) / 32
		chunks2 := (len2 + 31) / 32

		str1Bytes := make([]byte, 0, len1)
		for i := uint64(0); i < chunks1; i++ {
			chunkVal, loadErr := r.Memory.Load(addr1 + 1 + i)
			if loadErr != nil {
				err = fmt.Errorf("missing string chunk at address %d", addr1+1+i)
				break
			}
			chunk := make([]byte, 32)
			chunkVal.FillBytes(chunk)
			remaining := len1 - uint64(len(str1Bytes))
			if remaining > 32 {
				str1Bytes = append(str1Bytes, chunk...)
			} else {
				str1Bytes = append(str1Bytes, chunk[:remaining]...)
			}
		}

		str2Bytes := make([]byte, 0, len2)
		for i := uint64(0); i < chunks2; i++ {
			chunkVal, loadErr := r.Memory.Load(addr2 + 1 + i)
			if loadErr != nil {
				err = fmt.Errorf("missing string chunk at address %d", addr2+1+i)
				break
			}
			chunk := make([]byte, 32)
			chunkVal.FillBytes(chunk)
			remaining := len2 - uint64(len(str2Bytes))
			if remaining > 32 {
				str2Bytes = append(str2Bytes, chunk...)
			} else {
				str2Bytes = append(str2Bytes, chunk[:remaining]...)
			}
		}

		if err != nil {
			break
		}

		resultBytes := append(str1Bytes, str2Bytes...)
		resultLen := uint64(len(resultBytes))
		resultChunks := (resultLen + 31) / 32

		resultAddr := r.NextMemoryAddr
		err = r.Memory.Store(resultAddr, big.NewInt(int64(resultLen)))
		if err != nil {
			break
		}

		for i := uint64(0); i < resultChunks; i++ {
			chunkStart := i * 32
			chunkEnd := chunkStart + 32
			if chunkEnd > resultLen {
				chunkEnd = resultLen
			}
			chunk := make([]byte, 32)
			copy(chunk, resultBytes[chunkStart:chunkEnd])
			storeErr := r.Memory.Store(resultAddr+1+i, new(big.Int).SetBytes(chunk))
			if storeErr != nil {
				err = storeErr
				break
			}
		}

		if err != nil {
			break
		}

		r.NextMemoryAddr += 1 + resultChunks
		err = r.Stack.Push(big.NewInt(int64(resultAddr)))

	case types.STRLEN:
		ptr, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
			break
		}
		addr := ptr.Uint64()
		lenVal, loadErr := r.Memory.Load(addr)
		if loadErr != nil {
			err = errors.New("invalid string pointer")
			break
		}
		err = r.Stack.Push(lenVal)

	case types.EMITSTR:
		ptr, popErr := r.Stack.Pop()
		if popErr != nil {
			err = popErr
			break
		}
		addr := ptr.Uint64()

		lenVal, loadErr := r.Memory.Load(addr)
		if loadErr != nil {
			err = errors.New("invalid string pointer for emit")
			break
		}
		strLen := lenVal.Uint64()
		chunks := (strLen + 31) / 32

		strBytes := make([]byte, 0, strLen)
		for i := uint64(0); i < chunks; i++ {
			chunkVal, loadErr := r.Memory.Load(addr + 1 + i)
			if loadErr != nil {
				err = fmt.Errorf("missing string chunk at address %d", addr+1+i)
				break
			}
			chunk := make([]byte, 32)
			chunkVal.FillBytes(chunk)
			remaining := strLen - uint64(len(strBytes))
			if remaining > 32 {
				strBytes = append(strBytes, chunk...)
			} else {
				strBytes = append(strBytes, chunk[:remaining]...)
			}
		}

		if err == nil {
			fmt.Printf("AVM EMITSTR: %s (len=%d, hex=%x)\n", string(strBytes), len(strBytes), strBytes)
		}

	case types.STRINDEX:
		// String character access: str[index]
		// Stack: [string_addr, index] -> [char_string_addr]
		index, err1 := r.Stack.Pop()
		if err1 != nil {
			err = err1
			break
		}
		stringAddr, err2 := r.Stack.Pop()
		if err2 != nil {
			err = err2
			break
		}

		stringContent := r.readStringFromMemory(stringAddr.Uint64())
		indexInt := int(index.Int64())

		fmt.Printf("AVM STRINDEX: addr=%d index=%d string=\"%s\" (len=%d)\n",
			stringAddr.Uint64(), indexInt, stringContent, len(stringContent))

		if indexInt < 0 || indexInt >= len(stringContent) {
			fmt.Printf("AVM STRINDEX ERROR: index %d out of range for string length %d\n", indexInt, len(stringContent))
			err = fmt.Errorf("string index %d out of range for string of length %d", indexInt, len(stringContent))
			break
		}

		// Extract single character as string
		charStr := string(stringContent[indexInt])
		charAddr := r.storeStringInMemory(charStr)
		fmt.Printf("AVM STRINDEX RESULT: char=\"%s\" stored_at_addr=%d\n", charStr, charAddr)
		err = r.Stack.Push(big.NewInt(int64(charAddr)))

	case types.STRSLICE:
		// String slicing: str[start:end]
		// Stack: [string_addr, start, end] -> [slice_string_addr]
		endIndex, err1 := r.Stack.Pop()
		if err1 != nil {
			err = err1
			break
		}
		startIndex, err2 := r.Stack.Pop()
		if err2 != nil {
			err = err2
			break
		}
		stringAddr, err3 := r.Stack.Pop()
		if err3 != nil {
			err = err3
			break
		}

		stringContent := r.readStringFromMemory(stringAddr.Uint64())
		startInt := int(startIndex.Int64())
		endInt := int(endIndex.Int64())

		fmt.Printf("AVM STRSLICE: addr=%d start=%d end=%d string=\"%s\" (len=%d)\n",
			stringAddr.Uint64(), startInt, endInt, stringContent, len(stringContent))

		// Handle 999999 as "use string length"
		if endInt == 999999 {
			endInt = len(stringContent)
			fmt.Printf("AVM STRSLICE: converted end=999999 to len=%d\n", endInt)
		}

		// Validate slice bounds
		if startInt < 0 || startInt > len(stringContent) {
			fmt.Printf("AVM STRSLICE ERROR: start index %d out of range for string length %d\n", startInt, len(stringContent))
			err = fmt.Errorf("slice start index %d out of range for string of length %d", startInt, len(stringContent))
			break
		}
		if endInt < startInt || endInt > len(stringContent) {
			fmt.Printf("AVM STRSLICE ERROR: end index %d out of range (start=%d, len=%d)\n", endInt, startInt, len(stringContent))
			err = fmt.Errorf("slice end index %d out of range (start=%d, len=%d)", endInt, startInt, len(stringContent))
			break
		}

		// Extract slice as new string
		sliceStr := stringContent[startInt:endInt]
		sliceAddr := r.storeStringInMemory(sliceStr)
		fmt.Printf("AVM STRSLICE RESULT: slice=\"%s\" (len=%d) stored_at_addr=%d\n", sliceStr, len(sliceStr), sliceAddr)
		err = r.Stack.Push(big.NewInt(int64(sliceAddr)))

	case types.STRTOINT:
		// Convert string to integer
		// Stack: [string_addr] -> [integer_value]
		stringAddr, err1 := r.Stack.Pop()
		if err1 != nil {
			err = err1
			break
		}
		stringContent := r.readStringFromMemory(stringAddr.Uint64())
		fmt.Printf("AVM STRTOINT: addr=%d string=\"%s\"\n", stringAddr.Uint64(), stringContent)

		// Parse string to integer
		var result int64
		var negative bool

		if len(stringContent) == 0 {
			fmt.Printf("AVM STRTOINT ERROR: empty string\n")
			err = fmt.Errorf("cannot convert empty string to integer")
			break
		}

		// Check for negative sign
		startIdx := 0
		if stringContent[0] == '-' {
			negative = true
			startIdx = 1
			if len(stringContent) == 1 {
				fmt.Printf("AVM STRTOINT ERROR: invalid string \"-\"\n")
				err = fmt.Errorf("cannot convert \"-\" to integer")
				break
			}
		}

		// Parse digits
		result = 0
		for i := startIdx; i < len(stringContent); i++ {
			char := stringContent[i]
			if char < '0' || char > '9' {
				fmt.Printf("AVM STRTOINT ERROR: invalid character '%c' at position %d\n", char, i)
				err = fmt.Errorf("invalid character '%c' in string \"%s\"", char, stringContent)
				break
			}
			result = result*10 + int64(char-'0')
		}

		if err != nil {
			break
		}

		if negative {
			result = -result
		}

		fmt.Printf("AVM STRTOINT RESULT: \"%s\" -> %d\n", stringContent, result)
		err = r.Stack.Push(big.NewInt(result))

	case types.INTTOSTR:
		// Convert integer to string
		// Stack: [integer_value] -> [string_addr]
		intValue, err1 := r.Stack.Pop()
		if err1 != nil {
			err = err1
			break
		}

		intVal := intValue.Int64()
		stringRepr := fmt.Sprintf("%d", intVal)
		stringAddr := r.storeStringInMemory(stringRepr)

		fmt.Printf("AVM INTTOSTR: %d -> \"%s\" stored_at_addr=%d\n", intVal, stringRepr, stringAddr)
		err = r.Stack.Push(big.NewInt(int64(stringAddr)))

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
	case TSXDATA:
		// Push transaction data as string to memory and return address
		tsxDataStr := string(r.Context.TsxData)
		stringAddr := r.storeStringInMemory(tsxDataStr)
		err = r.Stack.Push(big.NewInt(int64(stringAddr)))
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

// readStringFromMemory reads a string from memory at the given address
// String format: [length, chunk1, chunk2, ...]
// Returns empty string if address doesn't contain a valid string
func (r *Runtime) readStringFromMemory(addr uint64) string {
	// Try to read the length from the first slot
	lenVal, err := r.Memory.Load(addr)
	if err != nil {
		return ""
	}

	length := lenVal.Uint64()
	if length == 0 {
		return ""
	}

	// Calculate number of 32-byte chunks needed
	chunks := (length + 31) / 32

	// Read string bytes from chunks
	stringBytes := make([]byte, 0, length)
	for i := uint64(0); i < chunks; i++ {
		chunkVal, loadErr := r.Memory.Load(addr + 1 + i)
		if loadErr != nil {
			return "" // Invalid string format
		}

		chunk := make([]byte, 32)
		chunkVal.FillBytes(chunk)

		// Only take the bytes we need from this chunk
		remaining := length - uint64(len(stringBytes))
		if remaining > 32 {
			stringBytes = append(stringBytes, chunk...)
		} else {
			stringBytes = append(stringBytes, chunk[:remaining]...)
		}
	}

	return string(stringBytes)
}

// storeStringInMemory stores a string in memory and returns the address
// String format: [length, chunk1, chunk2, ...]
func (r *Runtime) storeStringInMemory(s string) uint64 {
	stringBytes := []byte(s)
	byteLength := uint64(len(stringBytes))
	chunksNeeded := (byteLength + 31) / 32

	// Get the starting address
	addr := r.NextMemoryAddr

	// Store the length in the first slot
	r.Memory.Store(addr, big.NewInt(int64(byteLength)))

	// Store string data in chunks
	for i := uint64(0); i < chunksNeeded; i++ {
		start := i * 32
		end := start + 32
		if end > byteLength {
			end = byteLength
		}

		// Create a 32-byte chunk (padded with zeros if needed)
		chunk := make([]byte, 32)
		copy(chunk, stringBytes[start:end])

		// Convert to big.Int and store
		chunkBig := new(big.Int).SetBytes(chunk)
		r.Memory.Store(addr+1+i, chunkBig)
	}

	// Update next available address
	r.NextMemoryAddr = addr + 1 + chunksNeeded

	return addr
}
