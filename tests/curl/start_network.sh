#!/bin/bash

# Kill any existing nodes
pkill -f "go run.*main.go"
sleep 1

echo "Starting 3-node blockchain network..."

# Start seed node (Node 1)
echo "Starting seed node on HTTP:8372 P2P:9372..."
go run ../../cmd/node/main.go --http 8372 --p2p 9372 --id seed-node &
SEED_PID=$!
sleep 2

# Start Node 2 connecting to seed
echo "Starting node 2 on HTTP:8373 P2P:9373..."
go run ../../cmd/node/main.go --http 8373 --p2p 9373 --id node-2 --seeds "127.0.0.1:9372" &
NODE2_PID=$!
sleep 2

# Start Node 3 connecting to seed
echo "Starting node 3 on HTTP:8374 P2P:9374..."
go run ../../cmd/node/main.go --http 8374 --p2p 9374 --id node-3 --seeds "127.0.0.1:9372" &
NODE3_PID=$!
sleep 3

echo ""
echo "Network started! Nodes running:"
echo "  - Seed node: HTTP=8372, P2P=9372 (PID=$SEED_PID)"
echo "  - Node 2:    HTTP=8373, P2P=9373 (PID=$NODE2_PID)"  
echo "  - Node 3:    HTTP=8374, P2P=9374 (PID=$NODE3_PID)"
echo ""
echo "Test the network with:"
echo "  ./test_network.sh"
echo ""
echo "Stop the network with:"
echo "  ./stop_network.sh"
echo ""

# Keep script running
wait