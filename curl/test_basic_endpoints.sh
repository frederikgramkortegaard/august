#!/bin/bash
echo "=== Testing All Basic API Endpoints ==="
echo ""

# Check if server is running
if ! curl -s --connect-timeout 2 --max-time 2 http://localhost:8372/api/chain/height > /dev/null; then
    echo "Server not responding on port 8372"
    echo "Start your node with: go run cmd/node/main.go"
    exit 1
fi

echo "Server is running. Testing all endpoints..."
echo ""

# Test chain height
echo "1. Testing GET /api/chain/height"
curl -X GET http://localhost:8372/api/chain/height \
  --max-time 2 --connect-timeout 2 \
  | jq '.' 2>/dev/null || cat
echo -e "\n"

# Test chain head
echo "2. Testing GET /api/chain/head"
curl -X GET http://localhost:8372/api/chain/head \
  --max-time 2 --connect-timeout 2 \
  | jq '.' 2>/dev/null || cat
echo -e "\n"

# Get genesis hash for testing block retrieval
echo "3. Testing GET /api/blocks/{hash} (Genesis block)"
GENESIS_HASH=$(curl -s --max-time 2 --connect-timeout 2 http://localhost:8372/api/chain/head | jq -r '.header | @base64d | .[0:64] | @base64' 2>/dev/null)

# If we can't extract hash, use known genesis hash
if [ -z "$GENESIS_HASH" ] || [ "$GENESIS_HASH" = "null" ]; then
    GENESIS_HASH="004026e54da4429fe9d07056d03f454b3181ba3967af85fa90b1061d4220f8d9"
    echo "Using known genesis hash: $GENESIS_HASH"
else
    echo "Using extracted genesis hash: $GENESIS_HASH"
fi

curl -X GET http://localhost:8372/api/blocks/$GENESIS_HASH \
  --max-time 2 --connect-timeout 2 \
  | jq '.' 2>/dev/null || cat
echo -e "\n"

# Test invalid endpoints
echo "4. Testing error handling - Invalid block hash"
curl -X GET http://localhost:8372/api/blocks/invalid_hash \
  --max-time 2 --connect-timeout 2 \
  | jq '.' 2>/dev/null || cat
echo -e "\n"

echo "5. Testing error handling - Method not allowed"
curl -X DELETE http://localhost:8372/api/chain/height \
  --max-time 2 --connect-timeout 2 \
  | jq '.' 2>/dev/null || cat
echo -e "\n"

echo "=== Basic endpoint tests completed! ==="