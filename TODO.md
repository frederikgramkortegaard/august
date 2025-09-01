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
- [x] Orphan block pool
- [x] Concurrent connection management
- [x] Request-response correlation with timeout handling
- [x] Broadcast storm mitigation via block deduplication
- [x] Automatic orphan parent block requests
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

- [ ] Chain reorganization (switch to longer chain)
- [ ] Fork detection and resolution
- [ ] Proper longest chain rule (total work)
- [ ] Chain sync for new nodes (initial block download)
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

- [ ] Transaction mempool data structure
- [ ] Transaction validation and deduplication
- [ ] Fee-based priority ordering
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
- [ ] Headers-first synchronization
- [ ] Compact blocks
- [ ] Transaction batching
- [ ] Message compression
- [ ] Connection pooling

## Priority 7: Advanced Features

Extended functionality (future enhancements):

- [ ] Basic scripting
- [ ] Multi-signature support
- [ ] Time-locked transactions
- [ ] Smart contract support

## Priority 8: Infrastructure & Operations

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
