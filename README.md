# gocuria

A blockchain implementation in Go.

## Status

Working:
- [x] Basic blockchain data structures (blocks, transactions, accounts)
- [x] Ed25519 digital signatures
- [x] Proof of work mining with difficulty adjustment
- [x] Transaction validation (nonces, balances, signatures)
- [x] Chain validation and state management
- [x] Genesis block with initial coin supply
- [x] In-memory storage with ChainStore interface
- [x] Peer-to-peer networking
- [x] Block broadcasting and propagation
- [x] Peer discovery and management
- [x] Message protocol (handshake, blocks, transactions, peer sharing)
- [x] Headers-first chain synchronization protocol
- [x] Candidate chain system with lock-free concurrent evaluation
- [x] Chain reorganization (longest chain rule based on total work)
- [x] Fork detection and resolution
- [x] Initial Block Download (IBD) for syncing new/behind nodes
- [x] Request-response correlation with timeout handling
- [x] Broadcast storm mitigation (block deduplication)
- [x] Channel-based asynchronous coordination

Not implemented:
- [ ] Persistent storage (database backend)
- [ ] Transaction mempool
- [ ] Duplicate connection prevention
- [ ] Comprehensive test coverage

## Building

```
go build ./cmd/main.go
```

## Running

Single node:
```
./main --p2p=9372
```

Multiple nodes:
```
# Node 1
./main --p2p=9001 --id=node1

# Node 2
./main --p2p=9002 --id=node2 --seeds=localhost:9001

# Node 3
./main --p2p=9003 --id=node3 --seeds=localhost:9001,localhost:9002
```

## Structure

```
blockchain/           Core blockchain logic
  types.go           Block, Transaction, Account structures
  validation.go      Block and transaction validation
  mining.go          Proof of work implementation
  difficulty.go      Difficulty adjustment algorithm
  forge.go           Block creation and mining
  processing/        Block processing and orphan handling
  store/             Storage interface and memory implementation

p2p/                 Peer-to-peer networking
  server.go          P2P server and message handling
  discovery.go       Peer discovery mechanism
  messages.go        Protocol message types
  peers.go           Peer management
  reqresp/           Request-response correlation layer

node/                Full node implementation
  node.go            FullNode orchestration

tests/               Integration tests
  node_test.go       Basic node operations
  block_propagation_test.go  Block propagation testing
  recent_blocks_test.go      Deduplication testing
  header_first_discovery_test.go  Headers-first protocol testing
  longest_chain_resolution_test.go  Chain competition and resolution testing

cmd/                 Command-line entry point
```

## Protocol

The P2P protocol uses TCP with JSON-encoded messages. Message types:

- HANDSHAKE: Initial peer identification with height and node info
- NEW_BLOCK: Block announcement and propagation
- NEW_BLOCK_HEADER: Headers-first gossip protocol
- NEW_TX: Transaction broadcast
- REQUEST_BLOCK: Block request by hash
- REQUEST_BLOCKS: Batch block requests for range syncing
- REQUEST_HEADERS: Headers-only requests for validation
- REQUEST_CHAIN_HEAD: Chain head requests for sync detection
- REQUEST_PEERS/SHARE_PEERS: Peer discovery protocol
- PING/PONG: Connection keepalive

Features:
- Headers-first chain synchronization following Bitcoin's IBD design
- Candidate chain system for concurrent evaluation of competing chains
- Lock-free operations using sync.Map for high-performance P2P
- Atomic chain switching based on total work comparison
- Request-response correlation with unique message IDs
- Timeout handling for pending requests (default 30s)
- Broadcast storm mitigation via block deduplication
- Recent blocks tracking with 5-minute TTL
- Automatic cleanup of expired resources

Chain Synchronization:
The system implements Bitcoin-style Initial Block Download (IBD) with a three-phase
approach: 1) Header evaluation and validation, 2) Candidate chain creation with
isolated storage, 3) Atomic promotion of the best chain. Multiple chains can be
evaluated concurrently without blocking, and the system always chooses the chain
with the most cumulative work (longest chain rule).

## Testing

The project includes comprehensive testing infrastructure:

**Integration Tests** (`tests/`):
- Multi-node blockchain networks with real P2P communication
- Block propagation and chain synchronization testing
- Headers-first discovery protocol validation
- Longest chain resolution with competing chains
- Deduplication and broadcast storm mitigation

**Fuzzybot Testing Framework** (`testing/bot.go`):
- Continuous multi-node simulation with randomized mining intervals
- Automated transaction generation and block creation
- Real-time chain status monitoring and validation
- Mock helper functions for test data generation

Run fuzzybot simulation:
```bash
go run testing/bot.go
```

The fuzzybot framework creates 7 nodes (1 seed + 6 peers) that continuously mine blocks with random intervals between 10-120 seconds, creating realistic network conditions for testing chain synchronization, transaction validation, and network resilience.

## Development

See TODO.md for development roadmap.