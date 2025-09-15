# Network Setup Guide

August uses a decentralized P2P protocol for blockchain synchronization and transaction propagation across multiple nodes.

## Network Architecture

### Node Types
- **Seed Node**: Full node with Query API enabled
- **Mining Node**: Full node with mining enabled
- **Full Node**: Validates blocks and maintains chain state

### Communication
- **P2P Protocol**: TCP-based peer discovery and block/transaction relay
- **Query API**: HTTP endpoints for external tools (wallet, miner, explorer)

## Single Node Setup

### Basic Seed Node
```bash
go run cmd/seed/main.go --port 9372 --minerport 8080
```

**Parameters:**
- `--port 9372` - P2P networking port
- `--minerport 8080` - Query API HTTP port

**Features:**
- Full blockchain validation
- HTTP API for queries
- P2P networking (but no peers initially)

## Multi-Node Network

### 3-Node Network Example

**Terminal 1 - Seed Node:**
```bash
go run cmd/seed/main.go --port 9000 --minerport 8080 --datadir ./data/node0
```

**Terminal 2 - Node 1:**
```bash
go run cmd/seed/main.go --port 9001 --minerport 8081 --datadir ./data/node1 --seed localhost:9000
```

**Terminal 3 - Node 2:**
```bash
go run cmd/seed/main.go --port 9002 --minerport 8082 --datadir ./data/node2 --seed localhost:9000
```

### Network Discovery
- Node 1 connects to seed node (localhost:9000)
- Node 2 connects to seed node (localhost:9000)
- Nodes 1 and 2 discover each other through peer sharing
- Forms mesh topology: 0 ↔ 1 ↔ 2 ↔ 0

## Command Line Parameters

### Global Flags
- `--port` - P2P networking port (required)
- `--minerport` - HTTP Query API port (default: 8080)
- `--datadir` - Data directory for blockchain storage (default: ./data)

### Network Flags
- `--seed` - Seed peer address (format: host:port)
- `--maxpeers` - Maximum peer connections (default: 50)

### Mining Flags
- `--mine` - Enable mining on this node
- `--privkey` - Mining private key (for block rewards)

## Network Topologies

### Star Network (Hub and Spoke)
```
    Node1
      ↑
 Seed ← → Node2
      ↓
    Node3
```

**Setup:**
- Seed: `--port 9000`
- Node1: `--port 9001 --seed localhost:9000`
- Node2: `--port 9002 --seed localhost:9000`
- Node3: `--port 9003 --seed localhost:9000`

### Linear Chain
```
Node1 ← → Seed ← → Node2 ← → Node3
```

**Setup:**
- Seed: `--port 9000`
- Node1: `--port 9001 --seed localhost:9000`
- Node2: `--port 9002 --seed localhost:9000`
- Node3: `--port 9003 --seed localhost:9002`

### Full Mesh (For Small Networks)
Each node connects to all others:
```
Node1 ← → Node2
  ↑         ↓
  ↑         ↓
Node4 ← → Node3
```

## Mining Networks

### Distributed Mining Setup

**Node 1 (Seed + Miner):**
```bash
go run cmd/seed/main.go --port 9000 --minerport 8080 --mine --privkey <key1>
```

**Node 2 (Full Node + Miner):**
```bash
go run cmd/seed/main.go --port 9001 --minerport 8081 --seed localhost:9000 --mine --privkey <key2>
```

**External Miners:**
```bash
# Miner connecting to Node 1
./cmd/miner/miner --privkey <key3> --node localhost:8080

# Miner connecting to Node 2
./cmd/miner/miner --privkey <key4> --node localhost:8081
```

### Mining Pool Simulation
Multiple miners connecting to single node:
```bash
# Node
go run cmd/seed/main.go --port 9000 --minerport 8080

# Miners
./cmd/miner/miner --privkey <key1> --node localhost:8080 --id miner-alice
./cmd/miner/miner --privkey <key2> --node localhost:8080 --id miner-bob
./cmd/miner/miner --privkey <key3> --node localhost:8080 --id miner-charlie
```

## Network Synchronization

### Initial Block Download (IBD)
1. **Connect**: New node connects to seed peers
2. **Headers**: Downloads block headers first (headers-first sync)
3. **Blocks**: Downloads full blocks for validation
4. **Validation**: Validates complete chain from genesis
5. **Ready**: Begins normal operation and mining

### Ongoing Synchronization
1. **Block Relay**: New blocks propagated peer-to-peer
2. **Transaction Relay**: Mempool transactions shared
3. **Fork Resolution**: Longest chain rule resolves conflicts

## Monitoring Networks

### Node Status
Check if nodes are connected:
```bash
# Query chain info from different nodes
curl http://localhost:8080/chain-info  # Node 0
curl http://localhost:8081/chain-info  # Node 1
curl http://localhost:8082/chain-info  # Node 2
```

All nodes should report same `height` and `hash`.

### Peer Discovery
Check node logs for peer connections:
```
node-9001    Connected to peer: localhost:9000
node-9001    Discovered new peer: localhost:9002
node-9001    Active peer connections: 2
```

### Block Propagation
Watch blocks propagate across network:
```
node-9000    Block 0000c3d4 added to main chain
node-9000    Relaying block header 0000c3d4 to 2 peers
node-9001    Received block header 0000c3d4 from peer localhost:9000
node-9002    Received block header 0000c3d4 from peer localhost:9001
```

## Testing Network Scenarios

### Network Partition
Simulate network splits:

**Partition 1 (Nodes 0,1):**
```bash
go run cmd/seed/main.go --port 9000 --minerport 8080
go run cmd/seed/main.go --port 9001 --minerport 8081 --seed localhost:9000
```

**Partition 2 (Nodes 2,3):**
```bash
go run cmd/seed/main.go --port 9002 --minerport 8082
go run cmd/seed/main.go --port 9003 --minerport 8083 --seed localhost:9002
```

**Healing:**
Connect partitions by adding cross-links:
```bash
# Add node from partition 1 to partition 2
go run cmd/seed/main.go --port 9004 --seed localhost:9000,localhost:9002
```

### Node Failure Recovery
1. **Stop Node**: Kill one node process
2. **Network Adapts**: Remaining nodes continue
3. **Restart Node**: Restart stopped node
4. **Resync**: Node downloads missed blocks and rejoins

### High Latency Networks
Simulate slow networks with delays:
```bash
# Add network delay (Linux/Mac with tc command)
sudo tc qdisc add dev lo root handle 1: prio
sudo tc qdisc add dev lo parent 1:1 handle 10: netem delay 100ms
```

## Performance Optimization

### Connection Limits
```go
// Adjust in networking code
MaxPeers = 20  // Reduce for low-resource nodes
MaxPeers = 100 // Increase for high-capacity nodes
```

### Block Relay Speed
- **Headers First**: Download headers before full blocks
- **Parallel Download**: Fetch blocks from multiple peers
- **Compact Blocks**: Send only transaction hashes (future enhancement)

### Memory Usage
- **Block Cache**: Limit cached blocks in memory
- **Peer Buffers**: Optimize network buffer sizes
- **Database**: Use efficient storage (PebbleDB)

## Security Considerations

### Peer Validation
- Nodes validate all received blocks and transactions
- Invalid data results in peer disconnection
- DoS protection through rate limiting

### Eclipse Attacks
- Connect to multiple diverse peers
- Avoid connecting only to single seed
- Implement peer diversity requirements

### Sybil Resistance
- Proof-of-work provides natural sybil resistance
- Peer reputation could be added (future enhancement)

## Network Configuration

### Firewall Setup
Allow P2P connections:
```bash
# Linux iptables
iptables -A INPUT -p tcp --dport 9000-9010 -j ACCEPT

# macOS
# Add rules in System Preferences > Security & Privacy > Firewall
```

### Port Forwarding (NAT)
For nodes behind routers:
```
External Port 19000 → Internal 192.168.1.100:9000
External Port 19001 → Internal 192.168.1.101:9001
```

### Docker Network
```dockerfile
# Dockerfile
FROM golang:1.21
COPY . /app
WORKDIR /app
RUN go build -o node cmd/seed/main.go
EXPOSE 9000 8080
CMD ["./node", "--port", "9000", "--minerport", "8080"]
```

```yaml
# docker-compose.yml
version: '3'
services:
  node0:
    build: .
    ports: ["9000:9000", "8080:8080"]
    command: ["./node", "--port", "9000", "--minerport", "8080"]

  node1:
    build: .
    ports: ["9001:9000", "8081:8080"]
    command: ["./node", "--port", "9000", "--minerport", "8080", "--seed", "node0:9000"]
```

## Troubleshooting

### Connection Issues
```
Failed to connect to seed peer: connection refused
```
**Solutions:**
- Check seed node is running
- Verify port numbers match
- Check firewall settings
- Test with `telnet localhost 9000`

### Sync Problems
```
Chain synchronization failed: invalid block header
```
**Solutions:**
- Delete data directory and resync
- Check all nodes use same genesis block
- Verify no corrupted block data

### Peer Discovery Issues
```
No peers discovered after 60 seconds
```
**Solutions:**
- Check seed peer addresses
- Ensure seed nodes are reachable
- Verify network connectivity
- Try multiple seed addresses

### Fork Resolution
```
Chain reorganization detected: switching to new chain
```
**Normal**: Network resolving competing chains
**Monitor**: Check if all nodes converge to same chain

## Production Deployment

### Recommended Setup
- **3+ Nodes**: Minimum for redundancy
- **Diverse Locations**: Different data centers/regions
- **Monitoring**: Log aggregation and alerting
- **Backup**: Regular chain state backups
- **Security**: Encrypted connections (future enhancement)

### Scaling Considerations
- **Bandwidth**: ~100KB/block for propagation
- **Storage**: ~1MB per 1000 blocks (varies with transactions)
- **CPU**: Mining uses 1 core, validation minimal
- **Memory**: ~50-200MB per node

### Monitoring Tools
```bash
# Node health check
curl -f http://localhost:8080/chain-info || alert "Node down"

# Sync status check
HEIGHT=$(curl -s http://localhost:8080/chain-info | jq -r '.height')
echo "Current height: $HEIGHT"

# Peer connectivity check
grep "Active peer connections" node.log | tail -1
```