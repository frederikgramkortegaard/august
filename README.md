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
- [x] Orphan block handling with automatic parent requests
- [x] Request-response correlation with timeout handling
- [x] Broadcast storm mitigation (block deduplication)
- [x] Channel-based asynchronous coordination

Not implemented:
- [ ] Persistent storage (database backend)
- [ ] Transaction mempool
- [ ] Chain reorganization (longest chain rule)
- [ ] Fork resolution
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

cmd/                 Command-line entry point
```

## Protocol

The P2P protocol uses TCP with JSON-encoded messages. Message types:

- HANDSHAKE: Initial peer identification with height and node info
- NEW_BLOCK: Block announcement and propagation
- NEW_TX: Transaction broadcast
- REQUEST_BLOCK: Block request by hash
- REQUEST_PEERS/SHARE_PEERS: Peer discovery protocol
- PING/PONG: Connection keepalive

Features:
- Request-response correlation with unique message IDs
- Timeout handling for pending requests (default 30s)
- Broadcast storm mitigation via block deduplication
- Recent blocks tracking with 5-minute TTL
- Automatic cleanup of expired block hashes

Nodes maintain an orphan pool for blocks received out of order. When a block's
parent arrives, orphaned children are automatically connected to the chain.
Missing parent blocks are automatically requested from connected peers.

## Development

See TODO.md for development roadmap.