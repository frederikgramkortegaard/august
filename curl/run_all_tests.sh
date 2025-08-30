#!/bin/bash
echo "Running all blockchain API tests..."
echo "Make sure your node is running on port 8372 first!"
echo ""

# Check if server is running
if ! curl -s http://localhost:8372/api/chain/height > /dev/null; then
    echo "Server not responding on port 8372"
    echo "Start your node with: go run cmd/node/main.go"
    exit 1
fi

echo "Server is running, proceeding with tests..."
echo ""

# Run all test scripts
./curl/get_chain_height.sh
./curl/get_head_block.sh
./curl/get_genesis_by_hash.sh
./curl/post_genesis_block.sh

echo "All tests completed!"