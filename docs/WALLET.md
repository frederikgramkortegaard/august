# Wallet Guide

The August wallet is a command-line tool for managing accounts, sending transactions, and interacting with smart contracts.

## Installation

Build the wallet binary:
```bash
go build -o cmd/wallet/wallet ./cmd/wallet
```

## Key Management

### Generate New Keys
```bash
./cmd/keygen/keygen
```
Output:
```
Private key (64 bytes): 1a2b3c4d5e6f...
Public key  (32 bytes): 9a8b7c6d5e4f...
```

Save your private key securely - you'll need it for all wallet operations.

## Basic Commands

### Check Balance
```bash
./cmd/wallet/wallet --privkey <your_private_key> --node localhost:8080 balance
```

### Send Funds
```bash
./cmd/wallet/wallet --privkey <your_private_key> --node localhost:8080 send \
  --amount 1000000 \
  --to <recipient_public_key>
```

## Smart Contract Operations

### Deploy Contract
```bash
./cmd/wallet/wallet --privkey <your_private_key> --node localhost:8080 deploy \
  --amount 1000000 \
  --gas-limit 50000 \
  --gas-price 100
```

### Call Contract
```bash
./cmd/wallet/wallet --privkey <your_private_key> --node localhost:8080 call \
  --contract <contract_address> \
  --amount 0 \
  --gas-limit 25000 \
  --gas-price 100
```

## Parameters

### Global Flags
- `--privkey` - Your private key (64-byte hex string)
- `--node` - Node address (default: localhost:8080)

### Send Command
- `--amount` - Amount to send in Leaf units (required)
- `--to` - Recipient's public key (32-byte hex string) (required)

### Deploy Command
- `--amount` - Initial funds to send to contract (default: 0)
- `--gas-limit` - Maximum gas for deployment (default: 50000)
- `--gas-price` - Gas price per unit (default: 100)

### Call Command
- `--contract` - Contract address to call (32-byte hex string) (required)
- `--amount` - Funds to send with call (default: 0)
- `--gas-limit` - Maximum gas for execution (default: 50000)
- `--gas-price` - Gas price per unit (default: 100)

## Currency Units

- **Leaf** - Base unit (like wei in Ethereum)
- **AUG** - 1 AUG = 1,000,000 Leaf (10^6)

All wallet amounts are in Leaf units.

## Gas System

### Gas Calculation
Total transaction cost = `amount + (gas_used * gas_price)`

### Typical Gas Usage
- Basic transfer: ~21,000 gas
- Contract deployment: ~25,000-100,000 gas
- Contract call: ~21,000-50,000 gas

### Gas Price Guidelines
- Low priority: 50-100 gas price
- Normal priority: 100-200 gas price
- High priority: 200+ gas price

## Contract Development

### Modifying Contract Code

The wallet uses a hardcoded contract in the `GetContract()` function. To deploy different contracts:

1. Edit `cmd/wallet/main.go` around line 257
2. Modify the `GetContract()` function
3. Rebuild: `go build -o cmd/wallet/wallet ./cmd/wallet`

### Example Contract

The default contract is a simple counter:
- **Initialization**: Stores value 42 at address 1
- **Runtime**: Increments stored value on each call and emits result

```go
// Initialization code
initInstructions := []avm.Instruction{
    {Opcode: avm.PUSH, Value: big.NewInt(42)}, // Initial value
    {Opcode: avm.PUSH, Value: big.NewInt(1)},  // Storage address
    {Opcode: avm.PSTORE},                      // Store 42 at address 1
}

// Runtime code
runtimeInstructions := []avm.Instruction{
    {Opcode: avm.PUSH, Value: big.NewInt(1)}, // Storage address
    {Opcode: avm.PLOAD},                      // Load current value
    {Opcode: avm.PUSH, Value: big.NewInt(1)}, // Increment amount
    {Opcode: avm.ADD},                        // Add 1
    {Opcode: avm.DUP},                        // Duplicate for storage
    {Opcode: avm.PUSH, Value: big.NewInt(1)}, // Storage address
    {Opcode: avm.SWAP, Value: big.NewInt(1)}, // Swap to get value on top
    {Opcode: avm.PSTORE},                     // Store new value
    {Opcode: avm.EMIT},                       // Emit result
}
```

## Error Messages

The wallet provides detailed error messages for common issues:

### Balance Errors
```
insufficient balance: have 1000000, needs 2500000 (amount=100000, maxGas=2400000, deployment=0)
```

### Gas Errors
```
gas used 55000 exceeds gas limit 50000
```

### Validation Errors
```
invalid transaction nonce: expected 5, got 3
```

### Network Errors
```
failed to get balance: connection refused
```

## Troubleshooting

### Transaction Stuck in Mempool
- Check if nonce is correct (should be current nonce + 1)
- Ensure sufficient balance for amount + gas fees
- Verify gas price is competitive

### Invalid Nonce Error
- Wait for previous transactions to be mined
- Check current nonce with balance command
- Don't send multiple transactions rapidly

### Connection Issues
- Verify node is running on specified port
- Check node has Query API enabled
- Try different node address

## Examples

### Complete Workflow

1. **Generate keys:**
```bash
./cmd/keygen/keygen
# Save the output keys
```

2. **Check initial balance:**
```bash
./cmd/wallet/wallet --privkey <your_key> balance
```

3. **Deploy a contract:**
```bash
./cmd/wallet/wallet --privkey <your_key> deploy --amount 1000000
# Note the transaction hash
```

4. **Wait for mining, then find contract address:**
```bash
curl http://localhost:8080/chain-info
# Find recent blocks and check for contract deployment
```

5. **Call the contract:**
```bash
./cmd/wallet/wallet --privkey <your_key> call --contract <contract_address>
```

6. **Check contract storage:**
```bash
curl http://localhost:8080/contract/<contract_address>
```

### Multiple Transactions

Wait between transactions to avoid nonce conflicts:
```bash
./cmd/wallet/wallet --privkey <key> send --amount 1000 --to <addr1>
# Wait 30+ seconds for mining
./cmd/wallet/wallet --privkey <key> send --amount 2000 --to <addr2>
```