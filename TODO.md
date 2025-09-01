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

## Immediate Tasks

Peer Management: 
- [ ] Fix duplicate connection detection (prevent double connections)
- [ ] Connection state machine improvements

Chain management:
- [ ] Chain reorganization (switch to longer chain)
- [ ] Fork detection and resolution
- [ ] Transaction mempool
- [ ] Block validation caching

Storage:
- [ ] Database backend (replace in-memory)
- [ ] State persistence
- [ ] Chain pruning options

## Short Term (1-2 weeks)

Persistent storage:
- [ ] SQLite or BoltDB backend
- [ ] Block index
- [ ] UTXO set tracking
- [ ] State snapshots

Consensus improvements:
- [ ] Proper longest chain rule (total work)
- [ ] Fork handling
- [ ] Reorg implementation
- [x] Request missing blocks from peers (implemented for orphans)

## Medium Term (3-4 weeks)

Transaction pool:
- [ ] Mempool data structure
- [ ] Transaction validation and deduplication
- [ ] Fee-based priority
- [ ] Mempool synchronization between peers

Testing:
- [x] Basic multi-node integration tests
- [x] Block propagation tests
- [x] P2P deduplication tests
- [ ] Network partition tests
- [ ] Fork scenario tests
- [ ] Performance benchmarks

## Long Term

Performance:
- [ ] Parallel block validation
- [ ] Headers-first synchronization
- [ ] Compact blocks
- [ ] Transaction batching

Features:
- [ ] Basic scripting
- [ ] Multi-signature support
- [ ] Time-locked transactions

Infrastructure:
- [ ] Docker containerization
- [ ] Metrics and monitoring
- [ ] Configuration management
- [ ] Logging improvements

## Known Issues

- Division by zero in difficulty calculation during orphan tests
- Duplicate connection detection needs improvement
- No chain sync for new nodes (only receives new blocks)
- Missing comprehensive test coverage

## Design Decisions

The system uses a simple architecture:
- In-memory storage (for now)
- JSON message encoding (not optimal but simple)
- TCP for P2P (could use UDP for some messages)
- Single-threaded validation (could parallelize)

These choices prioritize simplicity and correctness over performance.
