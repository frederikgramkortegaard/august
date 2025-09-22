# August: Proof-of-Work Blockchain Implementation

**Go · Distributed Systems · Blockchain · Virtual Machines · Compiler Design**

A blockchain platform built from scratch in Go, featuring proof-of-work consensus, a custom virtual machine, and a high-level smart contract programming language. Implements Bitcoin-inspired consensus with Ethereum-style smart contracts and gas-metered execution.

## Project Overview

August is a blockchain platform implementing the technology stack from consensus protocols to high-level programming languages. The system combines Bitcoin's proof-of-work consensus model with Ethereum-style smart contracts, featuring a custom virtual machine (AVM) and the Marigold programming language.

The blockchain uses SHA-256 proof-of-work with dynamic difficulty adjustment and longest-chain fork resolution. Smart contracts execute on AVM, a stack-based bytecode runtime with 256-bit arithmetic and gas-metered execution. Marigold provides a high-level syntax that compiles to AVM bytecode through a compiler pipeline with lexer, parser, type checker, and code generator (code generator is WIP).

The P2P network implements decentralized peer discovery and headers-first synchronization without hardcoded seed nodes. The toolchain includes wallet, miner, compiler, and HTTP API for development and interaction.

## Architecture

```
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Marigold      │─▶│       AVM       │─▶│   Blockchain    │
│   Language      │  │  Virtual Machine│  │     Core        │
└─────────────────┘  └─────────────────┘  └─────────────────┘
         │                       │                   │
         ▼                       ▼                   ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Compiler      │  │  Gas Metering   │  │  P2P Network    │
│   Toolchain     │  │  & Storage      │  │   Protocol      │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

## Key Technical Achievements

### Blockchain Implementation
- **Consensus Protocol**: Proof-of-work with SHA-256 and dynamic difficulty adjustment maintaining target block times
- **Fork Resolution**: Longest-chain rule with cumulative work calculation and automatic chain reorganization
- **Transaction Model**: Account-based state with Ed25519 digital signatures and nonce-based replay protection
- **Mempool Management**: Fee-based priority queue with automatic cleanup and transaction validation pipeline

### Virtual Machine Design
- **Stack Architecture**: 256-bit arithmetic using Go's `big.Int` with configurable stack and memory limits
- **Instruction Set**: 35+ opcodes including arithmetic, memory, storage, control flow, and blockchain context operations
- **Gas System**: Per-instruction gas accounting with overflow protection and execution limits
- **Persistent Storage**: Contract state management with PSTORE/PLOAD instructions for key-value storage

### Programming Language (Marigold)
- **Type System**: Static typing with int, float, string, bool, arrays, and maps with full type checking
- **Control Structures**: High-level syntax with if/else, while loops, break/continue statements, and function definitions
- **Blockchain Integration**: Built-in access to blockchain context via @-prefixed variables (@caller, @balance, @timestamp, etc.)
- **Compiler Pipeline**: Complete lexer->parser->typechecker->codegen pipeline with comprehensive error reporting

### Networking & Distribution
- **Peer Discovery**: Decentralized bootstrap mechanism without hardcoded seed nodes
- **Message Protocol**: Custom TCP-based protocol with JSON serialization and request-response correlation
- **Chain Synchronization**: Headers-first initial block download with efficient peer coordination
- **Node Management**: Automatic connection handling, timeout management, and peer cleanup

## Quick Start

### Network Setup
```bash
# Start seed node with HTTP API
go run cmd/seed/main.go --port 9372 --minerport 8080

# Generate cryptographic keys
go run cmd/keygen/main.go

# Start mining node
go run cmd/miner/main.go --privkey <private_key> --node localhost:8080
```

### Basic Wallet Operations
```bash
# Check account balance
go run cmd/wallet/main.go --privkey <private_key> --node localhost:8080 balance

# Send AUG to another address
go run cmd/wallet/main.go --privkey <sender_key> --node localhost:8080 send \
  --amount 1000 --to <recipient_public_key>

# Check transaction status via HTTP API
curl http://localhost:8080/transaction/<transaction_hash>

# View current chain information
curl http://localhost:8080/chain-info

# Check account balance via HTTP API
curl http://localhost:8080/balance/<public_key_address>
```

### Smart Contract Development
```go
# Write a simple fungible token contract in Marigold
define init() : int {
  // Initial supply minted to deployer based on AUG sent
  persistent[@caller] = string(@callvalue)
  return 0
}

define call() : int {
  // Buy tokens with AUG
  if len(@tsxdata) == 3 && @tsxdata[:3] == "buy" {
    persistent[@caller] = string(int(persistent[@caller]) + @callvalue)
    return 0
  }

  // Transfer tokens to another address
  // Format: "transfer" + 64-char hex address + amount
  if len(@tsxdata) >= 73 && @tsxdata[:8] == "transfer" {
    recipient: string = @tsxdata[8:72]
    amount: int = int(@tsxdata[72:])

    // Validate positive amount
    if amount <= 0 {
      return 1
    }

    // Check sufficient balance
    if int(persistent[@caller]) < amount {
      return 1
    }

    // Transfer tokens
    persistent[@caller] = string(int(persistent[@caller]) - amount)
    persistent[recipient] = string(int(persistent[recipient]) + amount)
  }

  return 0
}
```
# Compile to AVM bytecode
go run cmd/marigold/main.go token.mg

# Deploy to blockchain
go run cmd/wallet/main.go --privkey <key> --node localhost:8080 deploy \
  --init empty.avmbc --body token.avmbc --amount 0 --gas-limit 100000

# Mint tokens by sending AUG to contract
go run cmd/wallet/main.go --privkey <key> --node localhost:8080 call \
  --contract <address> --amount 1000 --gas-limit 50000

# Query token balance
go run cmd/wallet/main.go --privkey <key> --node localhost:8080 call \
  --contract <address> --amount 0 --gas-limit 50000
```

## Language Features

### Marigold Syntax
```marigold
define transfer(to: string, amount: int) : bool {
    // Type declarations and initialization
    balance: int = 1000
    rates: map[string]float = {}
    active: bool = true

    // Control flow with blockchain context
    if @balance < amount {
        emit("Insufficient contract balance")
        return false
    }

    current: int = persistent[to]
    persistent[to] = current + amount

    emit("Transfer completed")
    return true
}

define main() : int {
    // Loop constructs with break/continue
    i: int = 0
    while i < 10 {
        if i % 2 == 0 {
            i = i + 1
            continue
        }
        emit(i)
        if i > 7 { break }
        i = i + 1
    }
    return 0
}
```

### Blockchain Context Variables
- `@caller` - Message sender address
- `@address` - Current contract address
- `@balance` - Contract's AUG balance
- `@timestamp` - Block timestamp
- `@height` - Current block number
- `@gasprice` - Gas price for transaction
- `@callvalue` - AUG sent with call
- `@origin` - Transaction originator
- `@difficulty` - Current mining difficulty
- `@coinbase` - Block miner address
- `@gaslimit` - Block gas limit

## AVM Instruction Set

### Core Operations
```
Stack:      PUSH <hex>, POP, DUP, SWAP
Arithmetic: ADD, SUB, MUL, DIV, AND, OR
Comparison: EQ, LT, GT, ISZERO
Memory:     MSTORE, MLOAD (temporary)
Storage:    PSTORE, PLOAD (persistent)
Control:    JUMP, JUMPC, STOP
```

### Blockchain Context Instructions
```
CALLER      - Push message sender address (2 gas)
ADDRESS     - Push contract address (2 gas)
BALANCE     - Push account balance (400 gas)
TIMESTAMP   - Push block timestamp (2 gas)
HEIGHT      - Push block number (2 gas)
GASPRICE    - Push gas price (2 gas)
CALLVALUE   - Push call value (2 gas)
```

## HTTP API

### Blockchain Queries
```bash
# Account information with balance and nonce
curl http://localhost:8080/balance/993fe6a36d19ed527890a7698e940c3c1c629ca06443837871cb0ed19435229b

# Transaction lookup with block information
curl http://localhost:8080/transaction/d1cef77df9dff5b5abcd1234567890abcdef1234567890abcdef1234567890ab

# Block details by hash
curl http://localhost:8080/block/000046f22dc6219e1234567890abcdef1234567890abcdef1234567890abcdef

# Current chain head and height
curl http://localhost:8080/chain-info

# Full chain state including account balances and contract storage
curl http://localhost:8080/chain-state

# Pending transactions ordered by fee
curl http://localhost:8080/mempool?limit=10

# Example responses:
# Balance: {"address": "993fe6a3...", "exists": true, "balance": 1000000, "nonce": 5}
# Chain Info: {"height": 42, "head_hash": "000046f2...", "difficulty": 12345}
```

## Testing Framework

### Comprehensive Test Suite
```bash
# Run all tests
go test ./tests/...

# Component-specific testing
go test ./tests/avm/...         # Virtual machine tests
go test ./tests/storage/...     # Persistence layer tests
go test ./tests/mempool/...     # Transaction pool tests
go test ./tests/contracts/...   # Smart contract integration
go test ./marigold/tests/...    # Programming language tests

# End-to-end integration testing
go test ./tests/e2e/single_node_manual -v
python3 scripts/deploy_and_call_contract.py
```

### Multi-Node Network Simulation
```bash
# Terminal 1: Primary node
go run cmd/seed/main.go --port 9372 --minerport 8080

# Terminal 2: Secondary node
go run cmd/seed/main.go --port 9373 --minerport 8081 --peer localhost:9372

# Terminal 3-4: Distributed mining
go run cmd/miner/main.go --privkey <key1> --node localhost:8080
go run cmd/miner/main.go --privkey <key2> --node localhost:8081
```

## Project Structure

```
august/
├── avm/                    # Virtual machine implementation
│   ├── bytecode.go         # Instruction definitions (35+ opcodes)
│   ├── runtime.go          # Stack-based execution engine
│   ├── gas.go              # Gas metering and pricing
│   └── parser.go           # Bytecode parsing and validation
├── blockchain/             # Core blockchain logic
│   ├── chain.go            # Block and transaction structures
│   ├── mining.go           # Proof-of-work implementation
│   ├── validation.go       # Consensus validation rules
│   └── crypto.go           # Ed25519 signatures and hashing
├── marigold/               # High-level programming language
│   ├── lexer.go            # Tokenization and lexical analysis
│   ├── parser.go           # AST construction and syntax parsing
│   ├── typechecker.go      # Static type analysis
│   ├── chainidents.go      # Blockchain context integration
│   └── tests/              # Language test suite
├── networking/             # P2P protocol implementation
├── storage/                # Persistent state management (PebbleDB)
├── cmd/                    # Command-line tools
│   ├── marigold/           # Language compiler
│   ├── wallet/             # Transaction and contract tools
│   ├── miner/              # Mining implementation
│   ├── seed/               # Full node with HTTP API
│   └── avm/                # Bytecode execution environment
└── tests/                  # Integration and unit tests
```

## Performance Characteristics

- **Block Time**: Configurable difficulty targeting (default ~10 seconds)
- **TPS**: Limited by block size and gas limits (similar to Ethereum)
- **Storage**: PebbleDB backend with atomic state transitions
- **Memory**: Bounded VM execution with configurable limits
- **Network**: TCP-based with connection pooling and timeout management

## Technical Implementation Notes

- **Atomic Updates**: Copy-validate-replace pattern for thread-safe chain operations
- **Gas Metering**: Prevents infinite loops and resource exhaustion in smart contracts
- **Type Safety**: Static analysis prevents runtime type errors in Marigold contracts
- **Fork Handling**: Automatic chain reorganization with total work comparison
- **State Management**: Efficient key-value storage with account and contract separation

---

**August** demonstrates a blockchain implementation from consensus protocols to high-level programming languages, showcasing distributed systems design, virtual machine architecture, and compiler construction in a unified platform.
