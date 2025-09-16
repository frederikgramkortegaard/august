# August - Blockchain Implementation

A complete blockchain implementation with smart contract virtual machine, written in Go from scratch. Features Bitcoin-style proof-of-work consensus, Ethereum-inspired smart contracts, and a custom stack-based virtual machine (AVM) for bytecode execution.

## Key Features

**Blockchain Core:**
- Proof-of-work consensus with SHA-256 and difficulty adjustment
- Longest-chain fork resolution with decentralized P2P networking
- Persistent storage with PebbleDB and fee-based transaction mempool
- HTTP query API for blockchain exploration

**Smart Contracts & Virtual Machine:**
- **AVM (August Virtual Machine)** - Custom stack-based bytecode runtime
- **Gas-metered execution** with 256-bit arithmetic and memory management
- **Persistent contract storage** with PSTORE/PLOAD instructions
- **Human-readable bytecode** - Write contracts in `.avmbc` text files
- **Contract deployment** with separate init and runtime code

**Developer Experience:**
- Command-line tools for wallet, mining, and contract deployment
- Bytecode parser with validation and round-trip conversion
- Comprehensive test suite and debugging scripts
- File-based contract development with hex notation

Built to understand distributed systems, virtual machines, consensus algorithms, and smart contract execution.

**Note**: This is a learning project, not production software.


## Quick Start

### Start a Network

Start a seed node with Query API:
```bash
go run cmd/seed/main.go --port 9372 --minerport 8080
```

Start a miner:
```bash
# Generate keys first
go run cmd/keygen/main.go

# Start mining with your private key
go run cmd/miner/main.go --privkey YOUR_PRIVATE_KEY --node localhost:8080
```

### Use the Wallet

Generate wallet keys:
```bash
go run cmd/keygen/main.go
```

Check balance:
```bash
go run cmd/wallet/main.go --privkey YOUR_PRIVATE_KEY --node localhost:8080 balance
```

Send transaction:
```bash
go run cmd/wallet/main.go --privkey SENDER_PRIVATE_KEY --node localhost:8080 send --amount 100 --to RECIPIENT_PUBLIC_KEY
```

### Query Blockchain Data

```bash
# Check account balance
curl http://localhost:8080/balance/993fe6a36d19ed527890a7698e940c3c1c629ca06443837871cb0ed19435229b

# Look up transaction
curl http://localhost:8080/transaction/d1cef77df9dff5b5abcd1234...

# Get block information
curl http://localhost:8080/block/000046f22dc6219e1234...

# View pending transactions
curl http://localhost:8080/mempool
```

### Try the Virtual Machine

Run the AVM bytecode demo:
```bash
go run cmd/avm/main.go
```

### Deploy and Call Smart Contracts

Deploy a contract using bytecode files:
```bash
go run cmd/wallet/main.go --privkey YOUR_PRIVATE_KEY --node localhost:8080 deploy \
  --init contracts/counter_init.avmbc --body contracts/counter_runtime.avmbc \
  --amount 1000000 --gas-limit 50000 --gas-price 100
```

Call a deployed contract:
```bash
go run cmd/wallet/main.go --privkey YOUR_PRIVATE_KEY --node localhost:8080 call --contract CONTRACT_ADDRESS --amount 0 --gas-limit 50000 --gas-price 100
```

### Parse and Validate Bytecode

Test bytecode parsing:
```bash
go run cmd/parser/main.go contracts/counter_runtime.avmbc
```

Parse custom bytecode from file:
```bash
echo "PUSH 0x2A PUSH 0x1 ADD EMIT" > my_contract.avmbc
go run cmd/parser/main.go my_contract.avmbc
```

### Run Tests

```bash
# Run end-to-end test (tests complete node/wallet/mining flow)
go test ./tests/e2e/single_node_manual -v

# All tests
go test ./tests/...

# Specific test suites
go test ./tests/avm/...         # Virtual machine tests
go test ./tests/storage/...     # Persistence tests
go test ./tests/mempool/...     # Transaction pool tests
go test ./tests/queryapi/...    # HTTP API tests

```

## Currency Units

- **1 leaf** = The smallest unit of currency (like 1 wei in Ethereum)
- **1 AUG** = 10^18 leaf (like 1 ETH = 10^18 wei)
- All amounts in transactions and balances are denominated in leaf

## Technical Features

- **Consensus**: Proof-of-work with SHA-256 and difficulty adjustment
- **Fork Resolution**: Longest-chain rule with total work calculation
- **Networking**: TCP-based P2P protocol with peer discovery and message routing
- **Persistence**: PebbleDB for blockchain storage with atomic operations
- **Mining**: CPU-based mining with configurable difficulty
- **Cryptography**: Ed25519 digital signatures for transaction authentication
- **Validation**: Full block and transaction validation pipeline
- **Transaction Mempool**: Fee-based priority queue with size limits and expiration
- **HTTP Query API**: RESTful endpoints for balance, transaction, and block lookups
- **AVM (August Virtual Machine)**: Stack-based bytecode virtual machine with memory and gas accounting
- **Testing**: Comprehensive test coverage including persistence, mempool, API, and AVM tests

## Architecture

```
avm/                 August Virtual Machine - Stack-based bytecode runtime
  ├── bytecode.go    Instruction definitions and validation
  ├── parser.go      Bytecode parsing and export functions
  ├── runtime.go     VM execution engine with stack and memory
  └── errors.go      VM-specific error types
blockchain/          Core blockchain logic (validation, mining, difficulty)
cmd/                 Command-line tools and executables
  ├── avm/           AVM bytecode execution demo
  ├── keygen/        Ed25519 key pair generation utility
  ├── miner/         Standalone CPU miner with mempool transaction inclusion
  ├── parser/        Bytecode parsing and validation tool
  ├── seed/          Seed node (full node + HTTP API server)
  └── wallet/        Transaction creation and balance checking tool
contracts/           Smart contract bytecode files (.avmbc format)
  ├── counter_init.avmbc    Example counter initialization code
  └── counter_runtime.avmbc Example counter runtime code
mempool/             Transaction pool with fee-based priority queue
miner/               Proof-of-work mining algorithms
networking/          P2P protocol implementation and message handling
node/                Full node orchestration and lifecycle management
  └── queryapi/      HTTP API package with shared types and handlers
    ├── api.go       HTTP request handlers
    └── types.go     Shared request/response structures
storage/             Persistent storage layer with PebbleDB
tests/               Comprehensive testing suite
  ├── avm/           AVM instruction and runtime tests
  ├── e2e/           End-to-end integration tests
  ├── storage/       Persistence and chain validation tests
  ├── mempool/       Transaction pool functionality tests
  └── queryapi/      HTTP API endpoint tests
```

## Key Implementation Details

- **Atomic Chain Updates**: Uses copy-validate-replace pattern for safe concurrent access
- **Headers-First Sync**: Follows Bitcoin's IBD protocol for efficient synchronization
- **Peer Discovery**: Decentralized bootstrap without hardcoded seed nodes
- **Block Index**: Hash-based block lookup for O(1) parent validation
- **Fork Handling**: Chain reorganization with total work comparison
- **Message Correlation**: Request-response tracking with timeout handling
- **Mempool Management**: Fee-based priority heap with automatic cleanup on block inclusion
- **Transaction Validation**: Pre-validation before mempool inclusion and revalidation after chain changes
- **RESTful API**: HTTP endpoints with proper error handling and flexible hash format support

## AVM (August Virtual Machine)

The August Virtual Machine is a stack-based bytecode runtime with 256-bit arithmetic and configurable resource limits.

### Quick Start

Run the AVM demo:
```bash
go run cmd/avm/main.go
```

Run AVM tests:
```bash
go test ./tests/avm/ -v
```

### Instruction Set

**Stack Operations:**
- `PUSH <hex_value>` - Push 256-bit hex value onto stack (3 gas)
- `POP` - Remove top stack item (2 gas)
- `DUP` - Duplicate top stack item (3 gas)
- `SWAP` - Swap top with second item (gets swap index from stack) (3 gas)

**Arithmetic:**
- `ADD`, `SUB`, `MUL`, `DIV` - Basic arithmetic (3-5 gas)
- `AND`, `OR` - Bitwise operations (3 gas)
- `EQ`, `LT`, `GT` - Comparison operations (3 gas)
- `ISZERO` - Zero check (2 gas)

**Memory:**
- `MSTORE` - Store value at memory address (20 gas)
- `MLOAD` - Load value from memory address (15 gas)

**Control Flow:**
- `JUMP` - Unconditional jump (gets target from stack) (8 gas)
- `JUMPC` - Conditional jump if second stack item is 1 (gets target from stack) (10 gas)
- `STOP` - Halt execution (0 gas)

**I/O:**
- `EMIT` - Output top stack value as hex (0 gas)

### Runtime Configuration

Runtime limits are configured in `config/config.go`:

```go
// AVM runtime limits (configured globally)
AVMMaxStackSize  = 1024  // Maximum stack depth
AVMMaxMemorySize = 1024  // Maximum memory slots

// Create runtime with gas limit and instructions
runtime := avm.NewRuntime(gasLimit, instructions)
gasUsed, err := runtime.StartExecution()
```

### Example Program

**Using Go API:**
```go
// Calculate 5 + 3 and emit result
instructions := []avm.Instruction{
    avm.MakePushInstruction("0x5"), // Push 5
    avm.MakePushInstruction("0x3"), // Push 3
    avm.MakeInstruction(avm.ADD),   // Add
    avm.MakeInstruction(avm.EMIT),  // Outputs: 0x08
}
```

**Using .avmbc bytecode files:**
```
PUSH 0x5 PUSH 0x3 ADD EMIT
```

**Parse and execute bytecode:**
```go
instructions, err := avm.ParseInstructionsFromString("PUSH 0x5 PUSH 0x3 ADD EMIT")
if err != nil {
    log.Fatal(err)
}
runtime := avm.NewRuntime(1000, instructions)
gasUsed, err := runtime.StartExecution()
```

### Features

- **256-bit arithmetic** using Go's `big.Int`
- **Gas accounting** with per-instruction costs
- **Stack overflow protection** with configurable limits
- **Memory bounds checking** with sparse map storage
- **Control flow validation** prevents invalid jumps
- **Comprehensive error handling** for all edge cases

## Smart Contracts

Smart contracts in August are AVM bytecode programs with persistent storage and gas-metered execution.

### Contract Structure

Contracts have two types of code:
- **Init Instructions**: Run once during deployment to initialize storage
- **Runtime Instructions**: Run every time the contract is called

### Persistent Storage

Contracts can store data permanently using:
- `PSTORE` - Store value at persistent storage address (1000 gas)
- `PLOAD` - Load value from persistent storage address (1000 gas)

Storage keys and values are 256-bit integers stored as strings.

### Example Contract

Simple counter that increments on each call:

**Init Code (runs once at deployment):**
```
PUSH 0x2A PUSH 0x1 PSTORE
```
*Initialize counter to 42 (0x2A) at storage address 1*

**Runtime Code (runs on each call):**
```
PUSH 0x1 PLOAD PUSH 0x1 ADD DUP PUSH 0x1 PSTORE EMIT
```
*Load counter, increment by 1, store back, and emit new value*

**File Structure:**
```
contracts/counter_init.avmbc    - Contains: PUSH 0x2A PUSH 0x1 PSTORE
contracts/counter_runtime.avmbc - Contains: PUSH 0x1 PLOAD PUSH 0x1 ADD DUP PUSH 0x1 PSTORE EMIT
```

### Contract Deployment

1. **Create Transaction**: Use wallet with `deploy` command
2. **Gas Payment**: Deployer pays base deployment fee + gas for init code execution
3. **Address Generation**: Contract address = hash(deployer_address + nonce)
4. **Initialization**: Init code runs to set up initial storage
5. **Storage**: Runtime code is stored for future calls

### Contract Calls

1. **Send Transaction**: Transaction with `To` field set to contract address
2. **Gas Metering**: Caller pays gas for runtime code execution
3. **Execution**: Runtime code runs with access to persistent storage
4. **State Changes**: Storage modifications are persisted after successful execution
5. **Events**: Contracts can emit data using `EMIT` instruction

### Contract Interaction Workflow

```bash
# 1. Deploy contract using bytecode files
go run cmd/wallet/main.go --privkey $PRIVATE_KEY --node localhost:8080 deploy \
  --init contracts/counter_init.avmbc --body contracts/counter_runtime.avmbc \
  --amount 1000000 --gas-limit 50000 --gas-price 100

# 2. Mine block to include deployment
go run cmd/miner/main.go --privkey $PRIVATE_KEY --node localhost:8080 --maxblocks 1

# 3. Call contract (increments counter)
go run cmd/wallet/main.go --privkey $PRIVATE_KEY --node localhost:8080 call \
  --contract $CONTRACT_ADDRESS --amount 0 --gas-limit 50000 --gas-price 100

# 4. Mine block to include call
go run cmd/miner/main.go --privkey $PRIVATE_KEY --node localhost:8080 --maxblocks 1

# 5. Check contract storage state
curl http://localhost:8080/chain-state | jq '.account_states["$CONTRACT_ADDRESS"].persistent'

# Alternative: Use debug script for automated deployment and testing
python3 scripts/deploy_and_call_contract.py        # Full workflow
python3 scripts/deploy_and_call_contract.py -step  # Interactive mode
```

### Bytecode Development

**Create custom contracts:**
```bash
# Create init code file
echo "PUSH 0x64 PUSH 0x2 PSTORE" > my_init.avmbc

# Create runtime code file
echo "PUSH 0x2 PLOAD PUSH 0x1 ADD DUP PUSH 0x2 PSTORE EMIT" > my_runtime.avmbc

# Test parsing
go run cmd/parser/main.go my_runtime.avmbc

# Deploy your custom contract
go run cmd/wallet/main.go --privkey $PRIVATE_KEY --node localhost:8080 deploy \
  --init my_init.avmbc --body my_runtime.avmbc \
  --amount 1000000 --gas-limit 50000 --gas-price 100
```

### Gas Costs

- **Contract Deployment**: 20,000 gas base fee + init code execution
- **Contract Call**: 21,000 gas base fee + runtime code execution
- **Storage Operations**: 1,000 gas per PSTORE/PLOAD
- **Failed Calls**: Gas is still consumed even if execution fails

## HTTP Query API

The node exposes a RESTful API for blockchain queries and miner operations:

### Blockchain Queries
- `GET /balance/{address}` - Get account balance and nonce (hex-encoded address)
- `GET /transaction/{hash}` - Look up transaction details (hex or base64 hash)
- `GET /block/{hash}` - Get block information (hex or base64 hash)
- `GET /chain-info` - Get current chain head information

### Mining Operations
- `GET /mempool?limit=N` - Get pending transactions ordered by fee
- `POST /submit-block` - Submit a mined block for validation

### Example Responses

**Balance Query:**
```json
{
  "address": "993fe6a36d19ed52...",
  "exists": true,
  "balance": 1000,
  "nonce": 5
}
```

**Transaction Lookup:**
```json
{
  "found": true,
  "transaction": { /* transaction data */ },
  "block_height": 42,
  "block_hash": "000046f22dc6219e...",
  "tx_index": 1
}
```

**Mempool Response:**
```json
{
  "count": 15,
  "transactions": [ /* ordered by fee, highest first */ ]
}
```

## Building

```bash
go mod tidy
go build -o august cmd/seed/main.go
```
