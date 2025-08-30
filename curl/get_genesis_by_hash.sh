#!/bin/bash
echo "=== Testing GET /api/blocks/:hash (Genesis Block) ==="
echo "Getting genesis block hash first..."

# Get genesis block hash from head endpoint
GENESIS_HASH=$(curl -s http://localhost:8372/api/chain/head | jq -r '.header.previous_hash // "GENESIS"')

# If we can't get the hash, use a known genesis hash pattern
if [ "$GENESIS_HASH" = "GENESIS" ] || [ "$GENESIS_HASH" = "null" ]; then
    echo "Using default genesis hash (all zeros)"
    GENESIS_HASH="004026e54da4429fe9d07056d03f454b3181ba3967af85fa90b1061d4220f8d9"
fi

echo "Using hash: $GENESIS_HASH"

curl -X GET http://localhost:8372/api/blocks/$GENESIS_HASH \
  -H "Content-Type: application/json" \
  | jq '.' 2>/dev/null || cat
echo -e "\n"