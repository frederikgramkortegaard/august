#!/bin/bash

echo "Testing 3-node blockchain network..."
echo "======================================"

# Test each node's HTTP API
for port in 8372 8373 8374; do
    echo ""
    echo "Testing Node on port $port:"
    
    # Check if node is responding
    if curl -s --connect-timeout 2 "http://localhost:$port/api/chain/height" >/dev/null; then
        echo "  ✓ Node is responding"
        echo "  Chain height:"
        curl -s "http://localhost:$port/api/chain/height" | jq .
        
        echo "  Chain head nonce:"
        curl -s "http://localhost:$port/api/chain/head" | jq -r '.header.nonce'
    else
        echo "  ✗ Node not responding"
    fi
done

echo ""
echo "Testing transaction validation on all nodes:"
echo "============================================"

# Test transaction validation on each node
# Valid base64 for 32-byte arrays (PublicKey) and 64-byte arrays (Signature)
COINBASE_TX='{"from":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","to":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","amount":1000,"nonce":0,"signature":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="}'

for port in 8372 8373 8374; do
    echo ""
    echo "Node $port transaction validation:"
    response=$(curl -s -X POST "http://localhost:$port/api/transactions" \
         -H "Content-Type: application/json" \
         -d "$COINBASE_TX")
    
    # Check if response is valid JSON
    if echo "$response" | jq . >/dev/null 2>&1; then
        echo "$response" | jq .
    else
        echo "Raw response: $response"
    fi
done

echo ""
echo "Network test complete!"
echo "All nodes should show the same chain height and genesis block."