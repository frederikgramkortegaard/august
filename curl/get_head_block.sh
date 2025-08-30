#!/bin/bash
echo "=== Testing GET /api/chain/head ==="
curl -X GET http://localhost:8372/api/chain/head \
  -H "Content-Type: application/json" \
  --max-time 2 \
  --connect-timeout 2 \
  | jq '.' 2>/dev/null || cat
echo -e "\n"