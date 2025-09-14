# TODO

## Completed Features ✅

1. **~~Implement transaction mempool and relay between nodes~~** ✅
   - ✅ Add transaction pool to collect pending transactions
   - ✅ Fee-based priority queue with size limits and expiration
   - ✅ Transaction validation before adding to mempool
   - ✅ Automatic cleanup when blocks are mined
   - ✅ HTTP endpoint for miners to fetch transactions
   - ⚠️ Transaction broadcast protocol between peers (partial - structure ready)

2. **~~Add HTTP query API for balance/transaction lookups~~** ✅
   - ✅ `/balance/<address>` - check account balance and nonce
   - ✅ `/transaction/<hash>` - lookup transaction details (confirmed + pending)
   - ✅ `/block/<hash>` - get block information and confirmations
   - ✅ Basic blockchain explorer functionality with comprehensive tests
   - ✅ Renamed to QueryAPI (supports both miners and blockchain queries)

## Next Steps

3. **Build basic wallet CLI for sending transactions**
   - Create keypair generation and management
   - Implement transaction creation and signing
   - Add balance checking functionality via QueryAPI
   - Simple CLI: `./wallet send --to=<address> --amount=100`
   - Transaction submission via HTTP to node

4. **Enhance P2P transaction relay**
   - Implement RelayTransaction function in networking package
   - Add transaction gossip protocol between peers
   - Validate and relay transactions across the network
   - Prevent transaction flooding and loops

5. **Add more blockchain explorer features**
   - `/blocks?limit=N&offset=M` - paginated block list
   - `/address/<addr>/transactions` - transaction history for address
   - `/stats` - network statistics (chain height, difficulty, etc.)
   - WebSocket support for real-time updates