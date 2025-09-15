# QueryAPI Reference

The August QueryAPI provides HTTP endpoints for blockchain queries, transaction submission, and miner operations.

## Base URL

Default: `http://localhost:8080`

Configure with `--query-port` flag when starting nodes.

## Authentication

No authentication required - all endpoints are public.

## Rate Limiting

No rate limiting implemented - use responsibly in production.

---

## Blockchain Query Endpoints

### GET /chain-info

Get current blockchain state information.

**Response:**
```json
{
  "height": 42,
  "hash": "000046f22dc6219e1a4b5c3d78a9b1c2d3e4f56789abcdef0123456789abcdef",
  "total_work": "12345678901234567890",
  "timestamp": 1634567890
}
```

**Fields:**
- `height` - Current blockchain height (number of blocks)
- `hash` - Latest block hash (64-character hex)
- `total_work` - Cumulative proof-of-work (decimal string)
- `timestamp` - Latest block timestamp (Unix seconds)

### GET /balance/{address}

Get account balance and nonce.

**Parameters:**
- `address` - Public key as 64-character hex string

**Example:**
```bash
curl http://localhost:8080/balance/993fe6a36d19ed527890a7698e940c3c1c629ca06443837871cb0ed19435229b
```

**Response:**
```json
{
  "address": "993fe6a36d19ed527890a7698e940c3c1c629ca06443837871cb0ed19435229b",
  "exists": true,
  "balance": 50000000,
  "nonce": 5
}
```

**Fields:**
- `address` - Queried address (hex)
- `exists` - Whether account exists on blockchain
- `balance` - Account balance in Leaf units
- `nonce` - Transaction nonce (number of transactions sent)

### GET /transaction/{hash}

Look up transaction details by hash.

**Parameters:**
- `hash` - Transaction hash (64-character hex or base64)

**Example:**
```bash
curl http://localhost:8080/transaction/d1cef77df9dff5b5abcd12345678901234567890abcdef0123456789abcdef01
```

**Response (Found):**
```json
{
  "found": true,
  "transaction": {
    "from": "993fe6a36d19ed527890a7698e940c3c1c629ca06443837871cb0ed19435229b",
    "to": "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
    "amount": 1000000,
    "gas_limit": 25000,
    "gas_price": 100,
    "nonce": 3,
    "timestamp": 1634567890
  },
  "block_height": 42,
  "block_hash": "000046f22dc6219e1a4b5c3d78a9b1c2d3e4f56789abcdef0123456789abcdef",
  "tx_index": 1
}
```

**Response (Pending):**
```json
{
  "found": true,
  "transaction": { /* transaction data */ },
  "status": "pending",
  "in_mempool": true
}
```

**Response (Not Found):**
```json
{
  "found": false,
  "hash": "d1cef77df9dff5b5abcd12345678901234567890abcdef0123456789abcdef01"
}
```

### GET /block/{hash}

Get block information by hash.

**Parameters:**
- `hash` - Block hash (64-character hex or base64)

**Example:**
```bash
curl http://localhost:8080/block/000046f22dc6219e1a4b5c3d78a9b1c2d3e4f56789abcdef0123456789abcdef
```

**Response:**
```json
{
  "found": true,
  "block": {
    "header": {
      "version": 1,
      "previous_hash": "000041a3b2c1d9e8f7654321098765432109876543210987654321098765432",
      "height": 42,
      "timestamp": 1634567890,
      "nonce": 185392,
      "merkle_root": "abc123def456789012345678901234567890abcdef0123456789abcdef012345",
      "bits": 486604799,
      "total_work": "12345678901234567890"
    },
    "transactions": [ /* array of transactions */ ]
  },
  "height": 42,
  "hash": "000046f22dc6219e1a4b5c3d78a9b1c2d3e4f56789abcdef0123456789abcdef",
  "tx_count": 3,
  "confirmations": 5
}
```

### GET /contract/{address}

Get contract information and persistent storage.

**Parameters:**
- `address` - Contract address as 64-character hex string

**Example:**
```bash
curl http://localhost:8080/contract/a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456
```

**Response:**
```json
{
  "found": true,
  "address": "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
  "balance": 1000000,
  "nonce": 0,
  "is_contract": true,
  "has_instructions": true,
  "storage_entries": 2,
  "persistent_data": {
    "1": "45",
    "2": "hello"
  },
  "code_hash": "def789abc012345678901234567890abcdef0123456789abcdef0123456789ab",
  "storage_root": "fed321cba098765432109876543210987654321098765432109876543210987"
}
```

**Fields:**
- `is_contract` - True if account has runtime code
- `storage_entries` - Number of persistent storage entries
- `persistent_data` - Key-value pairs from contract storage
- `code_hash` - Hash of contract runtime code
- `storage_root` - Merkle root of storage state

---

## Transaction Endpoints

### POST /submit-transaction

Submit a signed transaction to the mempool.

**Request Body:**
```json
{
  "from": "993fe6a36d19ed527890a7698e940c3c1c629ca06443837871cb0ed19435229b",
  "to": "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
  "amount": 1000000,
  "gas_limit": 25000,
  "gas_price": 100,
  "nonce": 4,
  "timestamp": 1634567890,
  "signature": {
    "r": "...",
    "s": "..."
  }
}
```

**Response (Success):**
```json
{
  "status": "submitted",
  "hash": "d1cef77df9dff5b5abcd12345678901234567890abcdef0123456789abcdef01"
}
```

**Response (Error):**
```http
HTTP/1.1 400 Bad Request
Transaction rejected: insufficient balance: has 1000, needs 2500000 (amount=1000000, maxGas=1500000, deployment=0)
```

### GET /mempool

Get pending transactions from mempool.

**Query Parameters:**
- `limit` - Maximum transactions to return (default: 100, max: 1000)

**Example:**
```bash
curl "http://localhost:8080/mempool?limit=10"
```

**Response:**
```json
{
  "count": 15,
  "transactions": [
    {
      "from": "993fe6a36d19ed527890a7698e940c3c1c629ca06443837871cb0ed19435229b",
      "to": "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
      "amount": 1000000,
      "gas_limit": 25000,
      "gas_price": 200,
      "nonce": 4
    }
  ]
}
```

**Note:** Transactions are ordered by fee (gas_limit * gas_price), highest first.

---

## Miner Endpoints

### GET /chain-state

Get current account states for transaction validation (used by miners).

**Response:**
```json
{
  "account_states": {
    "993fe6a36d19ed527890a7698e940c3c1c629ca06443837871cb0ed19435229b": {
      "address": "993fe6a36d19ed527890a7698e940c3c1c629ca06443837871cb0ed19435229b",
      "balance": 50000000,
      "nonce": 5,
      "instructions": [],
      "persistent": {},
      "storage_root": "0000000000000000000000000000000000000000000000000000000000000000",
      "code_hash": "0000000000000000000000000000000000000000000000000000000000000000"
    }
  },
  "account_count": 12
}
```

### POST /submit-block

Submit a mined block for validation and inclusion.

**Request Body:**
```json
{
  "header": {
    "version": 1,
    "previous_hash": "000041a3b2c1d9e8f7654321098765432109876543210987654321098765432",
    "height": 43,
    "timestamp": 1634567900,
    "nonce": 185392,
    "merkle_root": "abc123def456789012345678901234567890abcdef0123456789abcdef012345",
    "bits": 486604799,
    "total_work": "12345678901234567891"
  },
  "transactions": [ /* array of transactions */ ]
}
```

**Response:**
```json
{
  "status": "submitted"
}
```

---

## Error Codes

### 400 Bad Request
- Invalid transaction format
- Transaction validation failed
- Invalid hash format
- Missing required fields

### 404 Not Found
- Transaction not found
- Block not found
- Account doesn't exist

### 405 Method Not Allowed
- Wrong HTTP method for endpoint

### 500 Internal Server Error
- Database access failed
- Chain state corruption

---

## Hash Formats

The API accepts hashes in two formats:
- **Hex**: `000046f22dc6219e...` (64 characters)
- **Base64**: `AARm8i3GIZ4a...` (44 characters)

Always returns hashes in hex format.

---

## Usage Examples

### Check if Transaction Confirmed
```bash
#!/bin/bash
TX_HASH="d1cef77df9dff5b5abcd12345678901234567890abcdef0123456789abcdef01"
RESULT=$(curl -s "http://localhost:8080/transaction/$TX_HASH")
if echo "$RESULT" | jq -r '.in_mempool' | grep -q 'true'; then
    echo "Transaction pending in mempool"
elif echo "$RESULT" | jq -r '.found' | grep -q 'true'; then
    CONFIRMATIONS=$(echo "$RESULT" | jq -r '.block_height')
    echo "Transaction confirmed in block $CONFIRMATIONS"
else
    echo "Transaction not found"
fi
```

### Monitor Contract Storage
```bash
#!/bin/bash
CONTRACT="a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456"
while true; do
    VALUE=$(curl -s "http://localhost:8080/contract/$CONTRACT" | jq -r '.persistent_data["1"]')
    echo "$(date): Counter value = $VALUE"
    sleep 10
done
```

### Get Richest Accounts
```bash
# This would require iterating through all accounts
# The API doesn't provide a direct "richest accounts" endpoint
# You'd need to track accounts from transaction history
```

---

## Development Notes

### Adding Custom Endpoints
Modify `/node/queryapi/api.go` to add new handlers:

```go
mux.HandleFunc("/my-endpoint", handleMyEndpoint(node))
```

### Response Caching
No response caching implemented. Consider adding for frequently accessed data.

### Websocket Support
Not implemented. All endpoints are HTTP request-response only.

### Pagination
Only mempool endpoint supports limiting results. Block/transaction history endpoints don't support pagination.