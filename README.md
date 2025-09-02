# August - Educational Blockchain Implementation
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

August is an educational blockchain implementation written from scratch in Go, designed to explore and demonstrate various peer-to-peer networking concepts and blockchain technologies. This is a learning project that integrates proof-of-work consensus, Bitcoin-style chain synchronization, and advanced P2P networking patterns.

**Note**: This is not intended as production software or a commercial blockchain platform. Rather, it serves as a comprehensive study of blockchain architecture, P2P protocols, and distributed systems concepts through practical implementation.

:pushpin: [Design Decisions](https://github.com/user/gocuria/blob/master/DESIGN_DECISIONS.md)

:pushpin: [Development Roadmap](https://github.com/user/gocuria/blob/master/TODO.md)

:speech_balloon: This project demonstrates various distributed systems concepts including headers-first synchronization, concurrent chain evaluation, dynamic peer discovery, and network resilience testing. The fuzzybot framework showcases how to test P2P networks under realistic conditions with continuous node churn.

---

## Hosting a Full Node
To host a single full node, use the following command:
```bash
$ go build ./cmd/main.go
$ ./main --p2p=9372
```

For hosting a multi-node network:
```bash
# Node 1 (Bootstrap)
$ ./main --p2p=9001 --id=node1

# Node 2 
$ ./main --p2p=9002 --id=node2 --seeds=localhost:9001

# Node 3
$ ./main --p2p=9003 --id=node3 --seeds=localhost:9001,localhost:9002
```

This creates a multi-node network for studying distributed consensus, P2P networking behaviors, and blockchain synchronization patterns.

## Hosting a Lightweight Node
*[Work in Progress]*

Lightweight nodes will provide blockchain verification without storing the full chain history:
```bash
# Coming soon
$ ./main --light --p2p=9372 --trusted-peers=node1,node2
```

## Using the Wallet
*[Work in Progress]*

Command-line wallet interface for transaction management:
```bash
# Wallet operations (planned)
$ ./wallet create --name=mywallet
$ ./wallet balance --address=1A2B3C...
$ ./wallet send --to=1X2Y3Z... --amount=10.5
$ ./wallet history --address=1A2B3C...
```

## Implementation Overview

August implements a Bitcoin-inspired blockchain with the following characteristics:

```
• Consensus Mechanism: Proof-of-Work with SHA-256 hashing and dynamic difficulty adjustment targeting 10-minute block intervals
• Chain Selection: Longest chain rule based on cumulative proof-of-work (total work), not just block count
• Network Protocol: Headers-first synchronization following Bitcoin's Initial Block Download (IBD) design with candidate chain evaluation
• Transaction Model: UTXO-style account model with Ed25519 digital signatures and sequential nonce-based replay protection
• Mining: CPU-based proof-of-work with configurable difficulty and coinbase rewards (currently 50 coins per block)
• P2P Architecture: Fully decentralized peer discovery without permanent seed nodes, using TCP with JSON message encoding
```

## Code Structure
### Core Blockchain Components
```
blockchain/           Core blockchain logic
  types.go           Block, Transaction, Account structures
  validation.go      Block and transaction validation
  mining.go          Proof of work implementation
  difficulty.go      Difficulty adjustment algorithm
  forge.go           Block creation and mining
  store/             Storage interface and memory implementation
```

### P2P Networking
```
p2p/                 Peer-to-peer networking
  server.go          P2P server and message handling
  discovery.go       Peer discovery mechanism
  service.go         Headers-first chain synchronization
  messages.go        Protocol message types
  reqresp/           Request-response correlation layer
```

### Node Architecture
```
node/                Full node implementation
  node.go            FullNode orchestration and lifecycle management
```

## Testing Framework

August includes comprehensive testing infrastructure across multiple levels:

### Unit Testing
Core component testing with isolated validation:
```bash
# Run all unit tests
$ go test ./blockchain/...
$ go test ./p2p/...

# With coverage reporting
$ go test -cover ./...
```

Coverage includes:
- Block and transaction validation logic
- Cryptographic signature verification
- Difficulty adjustment algorithms
- Message serialization/deserialization
- Chain storage operations

### Integration Testing
Multi-component interaction testing:
```bash
# Run integration test suite
$ go test ./tests/...

# Specific test scenarios
$ go test ./tests/block_propagation_test.go
$ go test ./tests/longest_chain_resolution_test.go
```

Test scenarios include:
- Multi-node blockchain networks with real P2P communication
- Block propagation and chain synchronization
- Headers-first discovery protocol validation
- Longest chain resolution with competing chains
- P2P message deduplication and broadcast storm mitigation

### Fuzzybot / Live Network Testing
Continuous multi-node simulation under realistic network conditions:
```bash
# Run live network simulation
$ go run testing/bot.go

# Monitor network behavior
$ tail -f log.txt | grep "CHAIN STATUS"
```

The fuzzybot framework provides:
- **Dynamic Node Management**: Nodes join and leave every 2 minutes
- **Randomized Mining**: Mining intervals between 10-120 seconds per node
- **Transaction Generation**: Automatic transaction creation with proper nonce handling
- **Chain Synchronization**: Real-time testing of Initial Block Download (IBD)
- **Network Resilience**: Tests network survival without permanent seed nodes
- **Consensus Testing**: Validates longest chain rule under competing mining scenarios

## Protocol Features
The P2P protocol uses TCP with JSON-encoded messages implementing:

- **Headers-First Synchronization**: Bitcoin-style Initial Block Download
- **Candidate Chain System**: Concurrent evaluation of competing chains
- **Dynamic Peer Discovery**: Decentralized bootstrap without permanent seeds
- **Request-Response Correlation**: Timeout handling with unique message IDs
- **Broadcast Storm Mitigation**: Block deduplication with TTL management

### Message Types
- `HANDSHAKE`: Peer identification and height exchange
- `NEW_BLOCK_HEADER`: Headers-first gossip protocol
- `NEW_BLOCK`: Full block propagation
- `REQUEST_BLOCK`: Block requests by hash
- `REQUEST_CHAIN_HEAD`: Chain synchronization detection
- `REQUEST_PEERS/SHARE_PEERS`: Dynamic peer discovery
- `PING/PONG`: Connection keepalive

## Building from Source
Navigate to the project directory and build:
```bash
$ go mod tidy
$ go build ./cmd/main.go
```

For verbose debugging output:
```bash
$ go run ./cmd/main.go --p2p=9001 --id=debug --verbose
```

## Development Status
**Implemented Features**:
- [x] Core blockchain (blocks, transactions, mining, validation)
- [x] Peer-to-peer networking with full protocol suite
- [x] Headers-first chain synchronization
- [x] Dynamic peer discovery without permanent seeds
- [x] Comprehensive testing with fuzzybot framework
- [x] Concurrent chain evaluation and fork resolution

**In Progress**:
- [ ] Transaction mempool and relay protocol
- [ ] JSON-RPC API layer
- [ ] Persistent storage backend
- [ ] Client/wallet infrastructure
- [ ] Standalone mining processes