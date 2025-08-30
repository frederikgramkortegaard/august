#!/bin/bash
echo "=== Testing Sequential Block Submission ==="
echo "Make sure your node is running first!"
echo ""

# Check if server is running
if ! curl -s --connect-timeout 2 --max-time 2 http://localhost:8372/api/chain/height > /dev/null; then
    echo "Server not responding on port 8372"
    echo "Start your node with: go run cmd/node/main.go"
    exit 1
fi

echo "Server is running. Submitting blocks sequentially..."
echo ""

echo "Submitting block 1..."
./curl/post_block_1.sh || echo "Block 1 failed, continuing..."
sleep 1
echo ""

echo "Submitting block 2..."
./curl/post_block_2.sh || echo "Block 2 failed, continuing..."
sleep 1
echo ""

echo "Submitting block 3..."
./curl/post_block_3.sh || echo "Block 3 failed, continuing..."
sleep 1
echo ""

echo "Submitting block 4..."
./curl/post_block_4.sh || echo "Block 4 failed, continuing..."
sleep 1
echo ""

echo "Sequential block submission completed!"
echo "Check chain height:"
curl -s --connect-timeout 2 --max-time 2 http://localhost:8372/api/chain/height | jq '.' 2>/dev/null || cat
echo ""
