# TODO

## Next Steps

1. **Implement transaction mempool and relay between nodes**
   - Add transaction pool to collect pending transactions
   - Implement transaction broadcast protocol between peers
   - Validate and relay transactions across the network

2. **Build basic wallet CLI for sending transactions**
   - Create keypair generation and management
   - Implement transaction creation and signing
   - Add balance checking functionality
   - Simple CLI: `./wallet send --to=<address> --amount=100`

3. **Add HTTP query API for balance/transaction lookups**
   - `/balance/<address>` - check account balance
   - `/transaction/<hash>` - lookup transaction details
   - `/block/<hash>` - get block information
   - Basic blockchain explorer functionality