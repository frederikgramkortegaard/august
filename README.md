# gocuria

A simple blockchain implementation in Go.

## What works

- [x] Basic blockchain data structures (blocks, transactions, accounts)
- [x] Ed25519 digital signatures
- [x] Proof of work mining (configurable difficulty)
- [x] Transaction validation (nonces, balances, signatures)  
- [x] Chain validation and state management
- [x] Genesis block with initial coin supply
- [x] In-memory storage interface
- [x] HTTP API server
- [x] Block submission and retrieval via REST

## Planned

- [ ] Peer-to-peer networking between nodes
- [ ] Block broadcasting and synchronization
- [ ] Persistent storage (database)
- [ ] Transaction mempool
- [ ] Chain reorganization (longest chain rule)

## Usage

Start a node:
```bash
go run cmd/node/main.go
```

Test the API:
```bash
curl http://localhost:8372/api/chain/height
curl http://localhost:8372/api/chain/head
```

Generate test data:
```bash
go run cmd/generate-curl/main.go
./curl/test_basic_endpoints.sh
```

## Architecture

- `blockchain/` - Core blockchain logic and types
- `blockchain/store/` - Storage interface and implementations
- `api/` - HTTP server and handlers  
- `testing/` - Test data generation
- `curl/` - API testing scripts

That's it. Just a learning project to understand how blockchains work.