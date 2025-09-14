# August - Blockchain Implementation

A complete blockchain implementation written in Go from scratch. Features Bitcoin-style proof-of-work consensus, longest-chain fork resolution, decentralized P2P networking with peer discovery, persistent storage, fee-based transaction mempool, and HTTP query API for blockchain exploration.

Built to understand distributed systems concepts including consensus algorithms, network protocols, cryptographic validation, transaction management, and blockchain architecture.

**Note**: This is a learning project, not production software.

---

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

### Run Tests

```bash
# Run end-to-end test (tests complete node/wallet/mining flow)
go test ./tests/e2e/single_node_manual -v

# All tests
go test ./tests/...

# Specific test suites
go test ./tests/storage/...     # Persistence tests
go test ./tests/mempool/...     # Transaction pool tests
go test ./tests/queryapi/...    # HTTP API tests

```

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
- **Testing**: Comprehensive test coverage including persistence, mempool, and API tests

## Architecture

```
blockchain/          Core blockchain logic (validation, mining, difficulty)
cmd/                 Command-line tools and executables
  ├── keygen/        Ed25519 key pair generation utility
  ├── miner/         Standalone CPU miner with mempool transaction inclusion
  ├── seed/          Seed node (full node + HTTP API server)
  └── wallet/        Transaction creation and balance checking tool
mempool/             Transaction pool with fee-based priority queue
miner/               Proof-of-work mining algorithms
networking/          P2P protocol implementation and message handling
node/                Full node orchestration and lifecycle management
  └── queryapi/      HTTP API package with shared types and handlers
    ├── api.go       HTTP request handlers
    └── types.go     Shared request/response structures
storage/             Persistent storage layer with PebbleDB
tests/               Comprehensive testing suite
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
