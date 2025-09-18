package avm

import (
	"august/config"
	"errors"
	"fmt"
	"math/big"
)

// BlockchainContext contains immutable blockchain data needed for AVM execution
type BlockchainContext struct {
	BlockNumber uint64    // Current block height
	Timestamp   uint64    // Current block timestamp
	Difficulty  uint64    // Current block difficulty
	GasLimit    uint64    // Current block gas limit
	Coinbase    PublicKey // Current block miner/validator address
	ChainID     uint64    // Chain identifier

	// Transaction context
	Caller          PublicKey // Address of message sender (from CallTsx.From)
	Origin          PublicKey // Address of transaction originator
	GasPrice        uint64    // Price per gas unit (from CallTsx.GasPrice)
	CallValue       uint64    // Amount of currency sent in this call (from CallTsx.Amount)
	ContractAddress PublicKey // Address of the contract being executed

	// Deployment context (for contract calls)
	Deployer       PublicKey // Address that originally deployed this contract
	DeploymentTime uint64    // Block timestamp when contract was deployed
	DeploymentGas  uint64    // Gas price used for deployment

	// Account state snapshot (copied, not referenced)
	AccountStates map[PublicKey]*AccountState // Copy of relevant account states for lookups

	// Chain reference for historical block lookups (protected by RWMutex)
	ChainRef *Chain // Pointer to chain for BLOCKHASH opcode - use with chain.mu.RLock()
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
	Instructions []Instruction
	Persistent   map[string]string
	Context      *BlockchainContext // Immutable blockchain context data (includes AccountStates)
}

// NewBlockchainContext creates a BlockchainContext from a transaction and chain state
func NewBlockchainContext(tx *Transaction, chain *Chain, contractAddress PublicKey) BlockchainContext {
	// Lock chain for reading
	chain.mu.RLock()
	defer chain.mu.RUnlock()

	// Get current block (tip) information
	tip := chain.Tip
	if tip == nil {
		// Fallback to genesis or default values if no tip
		if len(chain.Blocks) > 0 {
			tip = chain.Blocks[len(chain.Blocks)-1]
		}
	}

	// Deep copy account states to avoid reference issues
	accountStatesCopy := make(map[PublicKey]*AccountState)
	for pubKey, state := range chain.AccountStates {
		if state != nil {
			// Create a deep copy of the account state
			stateCopy := *state // Shallow copy struct

			// Deep copy Instructions slice
			if state.Instructions != nil {
				stateCopy.Instructions = make([]Instruction, len(state.Instructions))
				for i, instr := range state.Instructions {
					stateCopy.Instructions[i] = Instruction{
						Opcode: instr.Opcode,
						Value:  nil,
					}
					if instr.Value != nil {
						valueCopy := *instr.Value
						stateCopy.Instructions[i].Value = &valueCopy
					}
				}
			}

			// Deep copy Persistent storage map
			if state.Persistent != nil {
				stateCopy.Persistent = make(map[string]string)
				for k, v := range state.Persistent {
					stateCopy.Persistent[k] = v
				}
			}

			accountStatesCopy[pubKey] = &stateCopy
		}
	}

	context := BlockchainContext{
		ChainID:         tx.ChainID,
		Caller:          tx.From,
		Origin:          tx.From, // For now, Origin same as Caller (no delegatecall yet)
		GasPrice:        tx.GasPrice,
		CallValue:       tx.Amount,
		ContractAddress: contractAddress,
		AccountStates:   accountStatesCopy,
		ChainRef:        chain,
	}

	// If this is a contract call (not deployment), we need to look up deployment transaction
	isDeployment := len(tx.Instructions) > 0 || len(tx.InitInstructions) > 0
	if !isDeployment {
		// This is a call to an existing contract - find the deployment transaction
		if contractState, exists := chain.AccountStates[contractAddress]; exists {
			if contractState.DeploymentBlockHash != (Hash32{}) && contractState.DeploymentTsxHash != (Hash32{}) {
				// Look up deployment block
				if deploymentBlock, blockFound := chain.GetBlockByHash(contractState.DeploymentBlockHash); blockFound {
					// Look up deployment transaction within that block
					if deploymentTx, txFound := deploymentBlock.FindTransactionByHash(contractState.DeploymentTsxHash); txFound {
						// Extract deployment context
						context.Deployer = deploymentTx.From
						context.DeploymentTime = deploymentBlock.Header.Timestamp
						context.DeploymentGas = deploymentTx.GasPrice
					}
				}
			}
		}
	}

	// Set block context if we have a tip
	if tip != nil {
		context.BlockNumber = tip.Header.Height
		context.Timestamp = tip.Header.Timestamp
		context.Difficulty = uint64(tip.Header.Bits) // Convert from compact format if needed
		context.GasLimit = tip.Header.GasLimit
		context.Coinbase = tip.Header.Beneficiary
	}

	return context
}

func NewRuntime(gasAvailable uint64, instructions []Instruction, context *BlockchainContext) *Runtime {
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
	Timestamp:       1640995200, // Jan 1, 2022
	Difficulty:      1000,
	GasLimit:        1000000,
	Coinbase:        PublicKey{}, // Zero address
	ChainID:         1,
	Caller:          PublicKey{}, // Zero address
	Origin:          PublicKey{}, // Zero address
	GasPrice:        1,
	CallValue:       0,
	ContractAddress: PublicKey{}, // Zero address
	Deployer:        PublicKey{}, // Zero address
	DeploymentTime:  1640995200,  // Same as genesis block timestamp
	DeploymentGas:   1,
	AccountStates:   make(map[PublicKey]*AccountState),
	ChainRef:        nil, // No chain reference for basic tests
}

// NewTestRuntime creates a Runtime with minimal blockchain context for testing
func NewTestRuntime(gasAvailable uint64, instructions []Instruction) *Runtime {
	return NewRuntime(gasAvailable, instructions, nil)
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
		//
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
