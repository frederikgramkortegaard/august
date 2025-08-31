# Multi-Node Network Testing

Simple scripts to test your distributed blockchain network with 3 nodes.

## Usage

1. **Start the network:**
   ```bash
   cd tests/curl
   chmod +x *.sh
   ./start_network.sh
   ```

2. **Test the network (in another terminal):**
   ```bash
   cd tests/curl
   ./test_network.sh
   ```

3. **Stop the network:**
   ```bash
   ./stop_network.sh
   ```

## What it does

- **start_network.sh**: Starts 3 blockchain nodes:
  - Seed node: HTTP port 8372, P2P port 9372
  - Node 2: HTTP port 8373, P2P port 9373 (connects to seed)
  - Node 3: HTTP port 8374, P2P port 9374 (connects to seed)

- **test_network.sh**: Tests all nodes via HTTP API:
  - Checks chain height on each node
  - Checks genesis block on each node  
  - Tests transaction validation on each node

- **stop_network.sh**: Cleanly stops all nodes

## Expected output

All nodes should show:
- Same chain height (1 after genesis)
- Same genesis block hash
- Working transaction validation
- P2P connections in the logs

The P2P discovery will connect the nodes together automatically!