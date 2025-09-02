# TODO

## Completed

Core blockchain:
- [x] Basic data structures (blocks, transactions)
- [x] Ed25519 signatures
- [x] Proof of work mining with difficulty adjustment
- [x] Transaction and block validation
- [x] Genesis block
- [x] In-memory chain storage

P2P networking:
- [x] TCP-based peer communication
- [x] Peer discovery with seed nodes
- [x] Message protocol (handshake, blocks, transactions, ping/pong)
- [x] Block broadcasting and relay
- [x] Candidate block system (replaced orphan block pool)
- [x] Concurrent connection management
- [x] Request-response correlation with timeout handling
- [x] Broadcast storm mitigation via block deduplication
- [x] Headers-first gossip protocol
- [x] Peer sharing protocol

Architecture:
- [x] Separation of processing logic from node orchestration
- [x] Thread-safe peer management
- [x] Modular component design
- [x] Channel-based asynchronous coordination
- [x] Request-response abstraction layer (reqresp package)
- [x] Service layer pattern for business logic
- [x] Function-based approach over OOP methods

## Priority 1: Chain Resolution & Consensus

Core functionality required for a working blockchain:

- [x] Chain reorganization (switch to longer chain)
- [x] Fork detection and resolution
- [x] Proper longest chain rule (total work)
- [x] Initial Block Download (IBD) for syncing new/behind nodes
- [x] Batch block requests ("give me blocks 4-8")
- [x] Headers-first synchronization
- [x] Chain sync for new nodes (proper sequential sync)
- [x] Candidate chain system with lock-free concurrent evaluation
- [ ] Fix division by zero in difficulty calculation during orphan tests

## Priority 2: Storage & State Management

Foundation for persistent blockchain state:

- [ ] Database backend (replace in-memory storage)
- [ ] SQLite or BoltDB backend
- [ ] Block index for fast lookups
- [ ] State persistence across restarts
- [ ] State snapshots for fast sync
- [ ] Chain pruning options

## Priority 3: Transaction Processing

Complete transaction lifecycle management:

- [ ] Transaction fees (fee field in Transaction struct)
- [ ] Transaction mempool data structure
- [ ] Transaction validation and deduplication
- [ ] Fee-based priority ordering
- [ ] Transaction broadcasting and relay protocol
- [ ] Mempool synchronization between peers
- [ ] Block validation caching

## Priority 4: Testing & Quality Assurance

Comprehensive testing before production use:

- [ ] Network partition tests
- [ ] Fork scenario tests
- [ ] Performance benchmarks
- [ ] Missing comprehensive test coverage
- [x] Basic multi-node integration tests
- [x] Block propagation tests
- [x] P2P deduplication tests
- [x] Headers-first discovery protocol tests
- [x] Longest chain resolution tests with multiple competing chains
- [x] Fuzzybot testing framework for continuous multi-node simulation
- [x] Random transaction generation and mining simulation
- [x] Mock helper functions for test data generation

## Priority 5: Networking Robustness

Network resilience features (not blocking core functionality):

- [ ] Fix duplicate connection detection (prevent double connections)
- [ ] Connection state machine improvements
- [ ] Handshake timeout handling
- [ ] Peer quality scoring and reputation
- [ ] Anti-DDoS protection mechanisms
- [ ] Connection retry logic with backoff
- [ ] Bandwidth throttling
- [ ] Peer blacklisting

## Priority 6: Performance Optimization

Performance improvements for scalability:

- [ ] Parallel block validation
- [ ] Compact blocks
- [ ] Transaction batching
- [ ] Message compression
- [ ] Connection pooling

## Priority 7: API & RPC Layer

Node interface for external applications:

- [ ] JSON-RPC server implementation
- [ ] REST API endpoints
- [ ] WebSocket real-time event streaming
- [ ] API authentication and rate limiting
- [ ] Transaction submission endpoint
- [ ] Balance and account state queries
- [ ] Block and chain information endpoints
- [ ] Network status and peer information
- [ ] Mining control and status APIs

## Priority 8: Advanced Features

Extended functionality (future enhancements):

- [ ] Basic scripting
- [ ] Multi-signature support
- [ ] Time-locked transactions
- [ ] Smart contract support

## Priority 9: Client & Wallet Infrastructure

User-facing tools for blockchain interaction:

- [ ] Wallet data structure and key management
- [ ] Transaction creation and signing utilities
- [ ] Balance queries and account state tracking
- [ ] Command-line wallet interface
- [ ] RPC client for node communication
- [ ] Transaction history and receipt tracking
- [ ] Multi-signature wallet support
- [ ] Wallet backup and recovery

## Priority 10: Mining Infrastructure

Standalone mining components:

- [ ] Separate miner process that connects to nodes
- [ ] Mining pool protocol and coordination
- [ ] Configurable mining difficulty and target adjustment
- [ ] Mining performance metrics and monitoring
- [ ] CPU mining optimization
- [ ] Mining reward distribution logic
- [ ] Remote mining via RPC/API
- [ ] Mining pool server implementation

## Priority 11: Infrastructure & Operations

Deployment and monitoring capabilities:

- [ ] Docker containerization
- [ ] Metrics and monitoring
- [ ] Configuration management
- [ ] Logging improvements
- [ ] Health check endpoints
- [ ] Administrative APIs

## Design Decisions

The system uses a simple architecture prioritizing correctness over performance:
- In-memory storage (temporary - will be replaced with persistent storage)
- JSON message encoding (simple but not optimal)
- TCP for P2P (reliable but could add UDP for some messages)
- Single-threaded validation (could parallelize later)
- Function-based service layer (not OOP methods)
- Service functions return completion channels for async tracking
