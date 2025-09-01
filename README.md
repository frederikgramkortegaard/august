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
- [x] Message protocol (handshake, blocks, transactions)
- [x] Orphan block handling

Not implemented:
- [ ] Persistent storage (database backend)
- [ ] Transaction mempool
- [ ] Chain reorganization (longest chain rule)
- [ ] Fork resolution
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
  processing/        Block processing and orphan handling
  store/             Storage interface and memory implementation

p2p/                 Peer-to-peer networking
  server.go          P2P server and message handling
  discovery.go       Peer discovery mechanism
  messages.go        Protocol message types
  peers.go           Peer management

node/                Full node implementation
  node.go            FullNode orchestration

cmd/                 Command-line entry point
```

## Protocol

The P2P protocol uses TCP with JSON-encoded messages. Message types:

- HANDSHAKE: Initial peer identification
- NEW_BLOCK: Block announcement
- NEW_TX: Transaction broadcast
- REQUEST_BLOCK: Block request by hash
- PING/PONG: Connection keepalive

Nodes maintain an orphan pool for blocks received out of order. When a block's
parent arrives, orphaned children are automatically connected to the chain.

## Development

See TODO.md for development roadmap.