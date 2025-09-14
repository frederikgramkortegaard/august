# August - Blockchain Implementation

A complete blockchain implementation written in Go from scratch. Features Bitcoin-style proof-of-work consensus, longest-chain fork resolution, decentralized P2P networking with peer discovery, and persistent storage.

Built to understand distributed systems concepts including consensus algorithms, network protocols, cryptographic validation, and blockchain architecture.

**Note**: This is a learning project, not production software.

---

## Quick Start

Start a seed node:
```bash
go run cmd/seed/main.go --port 9372 --minerport 8080
```

Start a miner:
```bash
go run cmd/miner/main.go --node localhost:8080
```

Run tests:
```bash
go test ./tests/storage/...
```

## Technical Features

- **Consensus**: Proof-of-work with SHA-256 and difficulty adjustment
- **Fork Resolution**: Longest-chain rule with total work calculation
- **Networking**: TCP-based P2P protocol with peer discovery and message routing
- **Persistence**: PebbleDB for blockchain storage with atomic operations
- **Mining**: CPU-based mining with configurable difficulty
- **Cryptography**: Ed25519 digital signatures for transaction authentication
- **Validation**: Full block and transaction validation pipeline
- **Testing**: Integration tests with persistence validation

## Architecture

```
blockchain/          Core blockchain logic (validation, mining, difficulty)
cmd/                 Node and miner executables
miner/               Proof-of-work mining algorithms
networking/          P2P protocol implementation and message handling
node/                Full node orchestration and lifecycle management
storage/             Persistent storage layer with PebbleDB
tests/               Integration and persistence testing
```

## Key Implementation Details

- **Atomic Chain Updates**: Uses copy-validate-replace pattern for safe concurrent access
- **Headers-First Sync**: Follows Bitcoin's IBD protocol for efficient synchronization
- **Peer Discovery**: Decentralized bootstrap without hardcoded seed nodes
- **Block Index**: Hash-based block lookup for O(1) parent validation
- **Fork Handling**: Chain reorganization with total work comparison
- **Message Correlation**: Request-response tracking with timeout handling

## Building

```bash
go mod tidy
go build -o august cmd/seed/main.go
```