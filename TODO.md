# TODO

## Phase 1: Transaction Lifecycle
**Goal**: Complete transaction system with mempool and fees

- [ ] Add fee field to Transaction struct
- [ ] Implement mempool with fee-based priority
- [ ] Transaction validation and relay protocol
- [ ] Pending transaction handling
- [ ] Fee estimation and minimum fees
- [ ] Transaction replacement policies
- [ ] Mempool synchronization between peers

## Phase 2: Test Coverage
**Goal**: Comprehensive testing for reliability

- [ ] Unit tests for all blockchain validation logic
- [ ] Unit tests for P2P message handling
- [ ] Integration tests for multi-node scenarios
- [ ] Transaction propagation tests
- [ ] Fork resolution and reorg tests
- [ ] Expand fuzzybot with adversarial behaviors
- [ ] Performance benchmarks

## Phase 3: Storage & Persistence
**Goal**: Move from in-memory to persistent storage

- [ ] Database backend (BoltDB or similar)
- [ ] Block and transaction indexing
- [ ] Account state persistence
- [ ] Node restart recovery
- [ ] Pruning and archival modes

## Phase 4: API & Wallet
**Goal**: External interfaces for using the blockchain

- [ ] JSON-RPC API server
- [ ] Basic wallet functionality
- [ ] Transaction creation and signing
- [ ] Balance and history queries
- [ ] CLI interface for wallet operations

## Phase 5: Mining Infrastructure
**Goal**: Decentralized mining

- [ ] Standalone miner process
- [ ] Mining work protocol
- [ ] Basic mining pool implementation
- [ ] Share validation and rewards

## Phase 6: Monitoring & Observability
**Goal**: Production-grade monitoring and insights

- [ ] Prometheus metrics exporter
- [ ] Grafana dashboards for visualization
- [ ] Chain metrics (height, hash rate, tx/s)
- [ ] Network metrics (peers, bandwidth, latency)
- [ ] Node health and performance metrics
- [ ] Alert rules for critical events

## Phase 7: Production Deployment
**Goal**: Ready for real-world use

- [ ] Network robustness improvements
- [ ] Peer reputation and banning
- [ ] Configuration file support
- [ ] Documentation and deployment guides

## Completed Features

### Core Blockchain
- [x] Block and transaction structures
- [x] Ed25519 cryptographic signatures
- [x] Proof-of-work consensus with difficulty adjustment
- [x] Genesis block initialization
- [x] Chain validation and reorganization
- [x] Longest chain rule (cumulative work)
- [x] In-memory chain storage (to be replaced)

### P2P Networking
- [x] TCP-based peer connections
- [x] Dynamic peer discovery
- [x] Headers-first synchronization (Bitcoin-style IBD)
- [x] Message protocol implementation
- [x] Request-response correlation layer
- [x] Broadcast storm mitigation
- [x] Candidate chain evaluation system
- [x] Lock-free concurrent operations

### Testing Infrastructure
- [x] Fuzzybot testing framework
- [x] Multi-node integration tests
- [x] Mock data generators
- [x] Basic unit test coverage
- [x] Network simulation with node churn

## Architecture Principles

- **Correctness over performance**: Get it working right first, optimize later
- **Clean separation of concerns**: Node, P2P, Blockchain layers
- **Testability**: Comprehensive testing at all levels
- **Real-world compatibility**: Follow Bitcoin's proven patterns where applicable
- **Educational value**: Code should be readable and well-documented
