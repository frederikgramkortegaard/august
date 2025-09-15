# Contract Development Guide

August supports smart contracts through the AVM (August Virtual Machine), a stack-based bytecode runtime with persistent storage capabilities.

## Quick Start

### Deploy Default Contract
```bash
# Deploy the built-in counter contract
./cmd/wallet/wallet --privkey <your_key> deploy --amount 1000000

# Wait for mining, get contract address from logs:
# "Deployed contract at: a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456"
```

### Call Contract
```bash
# Call contract to increment counter
./cmd/wallet/wallet --privkey <your_key> call --contract <contract_address>
```

### Check Contract Storage
```bash
# View persistent storage
curl http://localhost:8080/contract/<contract_address>
```

## Contract Architecture

### Contract Lifecycle
1. **Deployment**: `InitInstructions` run once to initialize storage
2. **Execution**: `Instructions` (runtime code) run on every call
3. **Storage**: Persistent key-value storage survives between calls

### Transaction Types
- **Contract Deployment**: `To` field empty, has `Instructions` and `InitInstructions`
- **Contract Call**: `To` field contains contract address, no instructions

## Built-in Counter Contract

The default wallet contract demonstrates basic storage operations:

### Initialization Code
```go
{Opcode: avm.PUSH, Value: big.NewInt(42)}, // Initial value
{Opcode: avm.PUSH, Value: big.NewInt(1)},  // Storage address
{Opcode: avm.PSTORE},                      // Store 42 at address 1
```

### Runtime Code
```go
{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Storage address
{Opcode: avm.PLOAD},                      // Load current value from address 1
{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Increment amount
{Opcode: avm.ADD},                        // Add 1 to current value
{Opcode: avm.DUP},                        // Duplicate result for emit
{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Storage address
{Opcode: avm.SWAP, Param: 1},            // Swap to get incremented value on top
{Opcode: avm.PSTORE},                     // Store incremented value at address 1
{Opcode: avm.EMIT},                       // Emit the new value
```

### Behavior
- **Initial State**: Storage[1] = 42
- **Each Call**: Increments storage[1] by 1, emits new value
- **Sequence**: 42 → 43 → 44 → 45 → ...

## AVM Instruction Set

### Stack Operations
- `PUSH <value>` - Push 256-bit value onto stack
- `POP` - Remove top stack item
- `DUP` - Duplicate top stack item
- `SWAP <n>` - Swap top with nth item (uses Param field)

### Arithmetic
- `ADD`, `SUB`, `MUL`, `DIV` - Basic arithmetic
- `AND`, `OR` - Bitwise operations
- `EQ`, `LT`, `GT` - Comparisons
- `ISZERO` - Zero check

### Memory (Temporary)
- `MSTORE` - Store in temporary memory
- `MLOAD` - Load from temporary memory

### Persistent Storage
- `PSTORE` - Store permanently (survives calls)
- `PLOAD` - Load from persistent storage

### Control Flow
- `JUMP <address>` - Unconditional jump
- `JUMPC <address>` - Conditional jump
- `STOP` - Halt execution

### I/O
- `EMIT` - Output top stack value

## Custom Contract Development

### 1. Modify GetContract Function

Edit `/cmd/wallet/main.go` around line 257:

```go
func GetContract() ([]avm.Instruction, []avm.Instruction) {
    // Your initialization code
    initInstructions := []avm.Instruction{
        // Set up initial storage
    }

    // Your runtime code
    runtimeInstructions := []avm.Instruction{
        // Contract logic
    }

    return initInstructions, runtimeInstructions
}
```

### 2. Example: Simple Storage

Store and retrieve a value:

```go
// Initialize with value 100
initInstructions := []avm.Instruction{
    {Opcode: avm.PUSH, Value: big.NewInt(100)},
    {Opcode: avm.PUSH, Value: big.NewInt(0)},
    {Opcode: avm.PSTORE},
}

// Return stored value when called
runtimeInstructions := []avm.Instruction{
    {Opcode: avm.PUSH, Value: big.NewInt(0)}, // Storage address
    {Opcode: avm.PLOAD},                      // Load value
    {Opcode: avm.EMIT},                       // Output value
}
```

### 3. Example: Calculator

Add two predefined numbers:

```go
initInstructions := []avm.Instruction{
    {Opcode: avm.PUSH, Value: big.NewInt(10)}, // First number
    {Opcode: avm.PUSH, Value: big.NewInt(0)},  // Address 0
    {Opcode: avm.PSTORE},

    {Opcode: avm.PUSH, Value: big.NewInt(20)}, // Second number
    {Opcode: avm.PUSH, Value: big.NewInt(1)},  // Address 1
    {Opcode: avm.PSTORE},
}

runtimeInstructions := []avm.Instruction{
    {Opcode: avm.PUSH, Value: big.NewInt(0)}, // Load first number
    {Opcode: avm.PLOAD},
    {Opcode: avm.PUSH, Value: big.NewInt(1)}, // Load second number
    {Opcode: avm.PLOAD},
    {Opcode: avm.ADD},                        // Add them
    {Opcode: avm.EMIT},                       // Output result
}
```

### 4. Example: Token Balance

Track token balances for addresses:

```go
// Initialize total supply
initInstructions := []avm.Instruction{
    {Opcode: avm.PUSH, Value: big.NewInt(1000000)}, // Total supply
    {Opcode: avm.PUSH, Value: big.NewInt(0)},       // Supply storage
    {Opcode: avm.PSTORE},
}

// For runtime: implement balance checks, transfers, etc.
// (This would require more complex logic with address handling)
```

## Gas Costs

### Instruction Costs
- `PUSH` - 3 gas
- `POP` - 2 gas
- `DUP` - 3 gas
- `SWAP` - 3 gas
- `ADD`, `SUB`, `MUL` - 3 gas
- `DIV` - 5 gas
- `MSTORE` - 20 gas
- `MLOAD` - 15 gas
- `PSTORE` - 20 gas (permanent storage)
- `PLOAD` - 15 gas (permanent storage)
- `JUMP` - 8 gas
- `JUMPC` - 10 gas
- `EMIT` - 0 gas

### Gas Guidelines
- **Simple contracts**: 25,000-50,000 gas limit
- **Storage-heavy contracts**: 50,000-100,000 gas limit
- **Complex logic**: 100,000+ gas limit

## Storage Patterns

### Key-Value Storage
```go
// Store: key → value
{Opcode: avm.PUSH, Value: big.NewInt(value)},
{Opcode: avm.PUSH, Value: big.NewInt(key)},
{Opcode: avm.PSTORE},

// Load: key → value
{Opcode: avm.PUSH, Value: big.NewInt(key)},
{Opcode: avm.PLOAD},
```

### Multiple Values
```go
// Store multiple values at different addresses
{Opcode: avm.PUSH, Value: big.NewInt(42)},  // value
{Opcode: avm.PUSH, Value: big.NewInt(1)},   // address 1
{Opcode: avm.PSTORE},

{Opcode: avm.PUSH, Value: big.NewInt(99)},  // value
{Opcode: avm.PUSH, Value: big.NewInt(2)},   // address 2
{Opcode: avm.PSTORE},
```

### Counters and Flags
```go
// Increment counter
{Opcode: avm.PUSH, Value: big.NewInt(0)}, // counter address
{Opcode: avm.PLOAD},                      // load current
{Opcode: avm.PUSH, Value: big.NewInt(1)}, // increment
{Opcode: avm.ADD},                        // add
{Opcode: avm.DUP},                        // duplicate for storage
{Opcode: avm.PUSH, Value: big.NewInt(0)}, // address
{Opcode: avm.SWAP, Param: 1},            // swap for storage
{Opcode: avm.PSTORE},                     // store new value
```

## Debugging Contracts

### Trace Execution
Enable AVM debug logging to trace instruction execution:

```go
// In avm/runtime.go, enable debug prints
fmt.Printf("Executing %s, gas: %d, stack: %v\n", ins.Opcode, gasUsed, stack)
```

### Check Storage State
Query contract storage after deployment/calls:
```bash
curl http://localhost:8080/contract/<address> | jq '.persistent_data'
```

### Monitor Gas Usage
Check transaction gas consumption:
- Deployment gas shows initialization cost
- Call gas shows runtime execution cost

### Test Patterns
```go
// Add debugging outputs
{Opcode: avm.DUP},   // Duplicate value for inspection
{Opcode: avm.EMIT},  // Emit intermediate values
// ... continue with logic
```

## Limitations

### Current Constraints
- No dynamic memory allocation
- No loops (use recursive calls)
- No external contract calls
- Fixed instruction set
- Storage keys are uint64 only

### Future Enhancements
- Contract-to-contract calls
- Events and logs
- Dynamic arrays
- String handling
- External data access (oracles)

## Security Considerations

### Gas Limits
Always set appropriate gas limits to prevent infinite loops.

### Storage Costs
Persistent storage is expensive - minimize unnecessary PSTORE operations.

### Validation
Contracts can't validate transaction signatures - this is done at protocol level.

### Reentrancy
Currently not possible since contracts can't call other contracts.

## Complete Examples

### 1. Deploy and Test Counter

```bash
# Deploy
./cmd/wallet/wallet --privkey <key> deploy --amount 1000000
# Note contract address: a1b2c3d4...

# Call multiple times
./cmd/wallet/wallet --privkey <key> call --contract a1b2c3d4...
./cmd/wallet/wallet --privkey <key> call --contract a1b2c3d4...
./cmd/wallet/wallet --privkey <key> call --contract a1b2c3d4...

# Check storage (should show incrementing values: 43, 44, 45...)
curl http://localhost:8080/contract/a1b2c3d4... | jq '.persistent_data'
```

### 2. Custom Contract Workflow

```bash
# 1. Edit GetContract() function with your code
vim cmd/wallet/main.go

# 2. Rebuild wallet
go build -o cmd/wallet/wallet ./cmd/wallet

# 3. Deploy contract
./cmd/wallet/wallet --privkey <key> deploy --amount 0

# 4. Test contract calls
./cmd/wallet/wallet --privkey <key> call --contract <address>
```

### 3. Storage Verification

```bash
# Deploy contract
CONTRACT=$(./cmd/wallet/wallet --privkey <key> deploy --amount 0 2>&1 | grep "Transaction Hash" | cut -d' ' -f3)

# Wait for mining
sleep 30

# Find contract address in recent blocks
curl -s http://localhost:8080/chain-info | jq -r '.hash' | xargs -I {} curl -s http://localhost:8080/block/{} | jq '.block.transactions[].to'

# Query contract storage
curl http://localhost:8080/contract/<address>
```