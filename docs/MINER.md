# Miner Guide

The August miner is a standalone CPU-based proof-of-work miner that connects to nodes via HTTP API to get mining jobs and submit blocks.

## Installation

Build the miner binary:
```bash
go build -o cmd/miner/miner ./cmd/miner
```

## Setup

### Generate Mining Keys
```bash
./cmd/keygen/keygen
```
Save your private key - this is where block rewards will be sent.

### Start Mining
```bash
./cmd/miner/miner --privkey <your_private_key> --node localhost:8080
```

## Parameters

- `--privkey` - Your private key (64-byte hex string) - **Required**
- `--node` - Node HTTP API address (default: localhost:8080)
- `--id` - Miner ID for logging (auto-generated if not provided)

## Mining Process

### 1. Get Chain Info
Miner polls the node for current chain state:
```
GET /chain-info
```

### 2. Fetch Mempool Transactions
Gets pending transactions for inclusion:
```
GET /mempool
```

### 3. Get Account States
Retrieves current blockchain state for transaction validation:
```
GET /chain-state
```

### 4. Validate Transactions
- Pre-executes transactions to check validity
- Calculates actual gas usage
- Skips invalid transactions with detailed logging

### 5. Create Block
- Builds coinbase transaction with block reward + gas fees
- Creates block header with merkle root
- Includes validated transactions

### 6. Mine Block
- Iterates through nonces to find valid proof-of-work
- Uses SHA-256 hashing with difficulty target
- Stops when hash meets target

### 7. Submit Block
Submits completed block to node:
```
POST /submit-block
```

## Mining Output

### Successful Mining Cycle
```
miner-1234    Current chain: height 42, head 00004a2f
miner-1234    Including 3 transactions from mempool
miner-1234    Current chain state has 15 accounts
miner-1234    Calculated total gas fees: 750000
miner-1234    Mining block at height 43...
miner-1234    Mined block 0000c3d4 (height 43, nonce 185392)
miner-1234    Block submitted successfully!
```

### Transaction Validation
```
miner-1234    Skipping invalid transaction: insufficient balance: has 1000, needs 2500000
```

### Contract Deployment
```
Deployed contract at: a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456
```

### Contract Execution
```
Executing contract at: a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456
Contract executed successfully, gas used: 24500
Applied 2 persistent storage updates
```

## Gas System

### Gas Calculation
The miner pre-executes all transactions to calculate exact gas usage:

1. **Validation**: Each transaction is executed against current state
2. **Gas Tracking**: Actual gas consumption is recorded
3. **Fee Calculation**: `gas_used * gas_price` for each transaction
4. **Coinbase**: Block reward + sum of all gas fees

### Gas Fees Distribution
- **Base Transfer**: ~21,000 gas
- **Contract Call**: ~21,000-50,000 gas
- **Contract Deployment**: ~25,000-100,000 gas
- **Storage Operations**: PSTORE (20 gas), PLOAD (15 gas)

## Mining Economics

### Block Rewards
- Current: 50 AUG (50,000,000 Leaf) per block
- Plus all transaction gas fees in the block

### Difficulty Adjustment
- Uses Bitcoin-style difficulty adjustment
- Target: ~10 minutes per block
- Adjusts based on recent block times

### Profitability Factors
1. **Hardware**: CPU performance affects hash rate
2. **Network**: More miners = higher difficulty
3. **Fees**: Include high-fee transactions for more profit
4. **Power Costs**: CPU mining uses significant power

## Configuration

### Easy Difficulty (for testing)
The miner uses `TestTargetCompact` by default for easy local mining.

### Production Settings
For mainnet-style difficulty, modify:
```go
TargetBits: blockchain.MainnetTargetCompact
```

## Troubleshooting

### Connection Issues
```
Failed to get chain info: connection refused
```
**Solutions:**
- Verify node is running and has Query API enabled
- Check node address and port
- Ensure firewall allows connections

### No Transactions
```
Including 0 transactions from mempool
```
**Normal**: Empty mempool means no pending transactions
**Check**: Use wallet to send transactions for testing

### Invalid Transactions
```
Skipping invalid transaction: invalid nonce
```
**Causes:**
- Sender account doesn't exist
- Insufficient balance for amount + gas
- Wrong nonce sequence
- Invalid signature

### Mining Too Slow
```
Mining block at height 15... (running for 5+ minutes)
```
**Solutions:**
- Reduce difficulty for testing
- Use faster CPU
- Check system load

### Block Rejected
```
Failed to submit block: HTTP error 400 Bad Request
```
**Causes:**
- Another miner found block first (stale block)
- Invalid block structure
- Transactions became invalid since mining started

## Advanced Usage

### Multiple Miners
Run multiple miners with different keys:
```bash
# Terminal 1
./cmd/miner/miner --privkey <key1> --node localhost:8080 --id miner-alice

# Terminal 2
./cmd/miner/miner --privkey <key2> --node localhost:8080 --id miner-bob
```

### Mining Pool Setup
Connect miners to different nodes in the network:
```bash
./cmd/miner/miner --privkey <key> --node node1.example.com:8080
./cmd/miner/miner --privkey <key> --node node2.example.com:8080
```

### Custom Mining Loop
Modify the mining interval in `main.go`:
```go
// Current: 30 second delay between attempts
time.Sleep(30 * time.Second)

// Faster: 10 second delay
time.Sleep(10 * time.Second)
```

## Performance Tips

### Optimize for Fees
The miner automatically includes highest-fee transactions first from the mempool.

### Monitor Logs
Watch for:
- Transaction validation errors
- Gas fee calculations
- Block submission status
- Network connectivity issues

### Resource Usage
- **CPU**: Mining uses one core at 100%
- **Memory**: Minimal (~10-50MB)
- **Network**: Periodic HTTP requests to node
- **Disk**: None (stateless miner)

## Integration with Monitoring

### Metrics to Track
- Blocks mined per hour
- Gas fees collected
- Transaction inclusion rate
- Stale block rate
- Average mining time

### Log Analysis
```bash
# Count successful blocks
grep "Block submitted successfully" miner.log | wc -l

# Track gas fees
grep "Calculated total gas fees" miner.log
```

## Development

### Custom Transaction Selection
Modify `getMempoolTransactions()` to implement custom transaction filtering or sorting logic.

### Different Mining Algorithms
Replace the proof-of-work algorithm in the miner package to experiment with different consensus mechanisms.