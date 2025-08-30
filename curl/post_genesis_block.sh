#!/bin/bash
echo "=== Testing POST /api/blocks (Genesis Block - Should Fail) ==="
curl -X GET http://localhost:8372/api/chain/head | jq -r '.header' > /tmp/genesis_header.json 2>/dev/null

curl -X POST http://localhost:8372/api/blocks \
  -H "Content-Type: application/json" \
  -d '{
    "header": {
      "version": 1,
      "previous_hash": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
      "timestamp": 1234567890,
      "nonce": 0,
      "merkle_root": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
    },
    "transactions": []
  }' \
  | jq '.' 2>/dev/null || cat
echo -e "\n"