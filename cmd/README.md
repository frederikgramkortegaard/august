# August Blockchain - Command Line Tools

This directory contains executable commands for running the August blockchain network.

## Available Commands

### 1. Seed Node (`seed/main.go`)
A dedicated seed node that other nodes connect to for bootstrapping.

```bash
go run cmd/seed/main.go --port 9371
```

**Flags:**
- `--port`: Network port to listen on (default: 9372)
- `--id`: Node ID (auto-generated if not provided)
- `--dbname`: Database name (default: seeddb)

### 2. Miner (`miner/main.go`)
A simple miner that connects to a node and mines blocks.

```bash
go run cmd/miner/main.go --node localhost:9372
```

**Flags:**
- `--node`: Node address to mine for (default: localhost:9372)
- `--id`: Miner ID (auto-generated if not provided)

## Example: Running a Mining Network

### Terminal 1: Start Seed Node
```bash
go run cmd/seed/main.go --port 9371 --id seed1
```

### Terminal 2: Start Additional Node (connects to seed)
```bash
go run cmd/seed/main.go --port 9372 --id node2 --dbname "node2db"
# Note: You can start more seed nodes, they will find each other through peer discovery
```

### Terminal 3: Start Miner
```bash
go run cmd/miner/main.go --node localhost:9371 --id miner1
```

## What You'll See

1. **Seed nodes** start and find each other through peer discovery
2. **Miner** connects to a seed node, gets chain info, mines blocks
3. **Mined blocks** get validated and propagated through P2P network
4. **All nodes** stay synchronized with the same blockchain

## Persistence

Each node stores its blockchain in a Pebble database. You can stop/restart nodes and they'll reload their chain from disk.

## Miner Details

The miner:
- Connects to a node via P2P (not HTTP)
- Requests current chain head info
- Creates coinbase transaction (block reward to miner)
- Mines the block (finds valid nonce)
- Submits via `MessageTypeSubmitBlock`
- Gets acknowledgment and repeats

This demonstrates a real Bitcoin-style mining pool setup where miners don't need to run full nodes!