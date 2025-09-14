package node

import (
	"august/blockchain"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// StartQueryAPI starts the HTTP API server for miners and blockchain queries (completely separate from P2P)
func (n *FullNode) StartQueryAPI(port int) error {
	// Create a new ServeMux to avoid conflicts when running multiple instances
	mux := http.NewServeMux()

	// Miner endpoints
	mux.HandleFunc("/chain-info", n.handleChainInfo)
	mux.HandleFunc("/submit-block", n.handleSubmitBlock)
	mux.HandleFunc("/mempool", n.handleGetMempool)

	// Query API endpoints for blockchain explorer functionality
	mux.HandleFunc("/balance/", n.handleBalance)
	mux.HandleFunc("/transaction/", n.handleTransaction)
	mux.HandleFunc("/block/", n.handleBlock)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting Query API server on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

// GET /chain-info - Returns the current chain head info
func (n *FullNode) handleChainInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	chainHead := n.GetChainHead()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chainHead)
}

// POST /submit-block - Submit a mined block
func (n *FullNode) handleSubmitBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var block blockchain.Block
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		http.Error(w, fmt.Sprintf("Invalid block format: %v", err), http.StatusBadRequest)
		return
	}

	// Process the block asynchronously (don't block the HTTP connection)
	go func() {
		<-n.ProcessBlock(&block)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "submitted"})
}

// GET /mempool - Returns pending transactions from mempool (highest fee first)
func (n *FullNode) handleGetMempool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get limit from query parameter (default: 100)
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
			if limit > 1000 {
				limit = 1000 // Cap at 1000
			}
			if limit < 1 {
				limit = 1 // Minimum 1
			}
		}
	}

	// Get transactions from mempool
	transactions := n.Mempool.GetTransactions(limit)

	response := map[string]interface{}{
		"count":        len(transactions),
		"transactions": transactions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GET /balance/<address> - Check account balance
func (n *FullNode) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract address from URL path
	path := strings.TrimPrefix(r.URL.Path, "/balance/")
	if path == "" {
		http.Error(w, "Address required", http.StatusBadRequest)
		return
	}

	// Parse address (assume hex encoding)
	addressBytes, err := hex.DecodeString(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid address format: %v", err), http.StatusBadRequest)
		return
	}

	if len(addressBytes) != 32 {
		http.Error(w, "Address must be 32 bytes", http.StatusBadRequest)
		return
	}

	// Convert to PublicKey
	var publicKey blockchain.PublicKey
	copy(publicKey[:], addressBytes)

	// Get current chain state
	chain, err := n.Store.GetChain()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get chain state: %v", err), http.StatusInternalServerError)
		return
	}

	// Look up account state
	accountState, exists := chain.AccountStates[publicKey]

	response := map[string]interface{}{
		"address": path,
		"exists":  exists,
	}

	if exists {
		response["balance"] = accountState.Balance
		response["nonce"] = accountState.Nonce
	} else {
		response["balance"] = 0
		response["nonce"] = 0
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GET /transaction/<hash> - Lookup transaction details
func (n *FullNode) handleTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract hash from URL path
	path := strings.TrimPrefix(r.URL.Path, "/transaction/")
	if path == "" {
		http.Error(w, "Transaction hash required", http.StatusBadRequest)
		return
	}

	// Parse hash (support both hex and base64)
	var targetHash blockchain.Hash32
	var err error

	// Try hex first (more common for blockchain hashes)
	hashBytes, err := hex.DecodeString(path)
	if err != nil {
		// Try base64
		hashBytes, err = base64.StdEncoding.DecodeString(path)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid hash format (tried hex and base64): %v", err), http.StatusBadRequest)
			return
		}
	}

	if len(hashBytes) != 32 {
		http.Error(w, fmt.Sprintf("Hash must be 32 bytes, got %d bytes from '%s'", len(hashBytes), path), http.StatusBadRequest)
		return
	}

	copy(targetHash[:], hashBytes)

	// Get current chain
	chain, err := n.Store.GetChain()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get chain: %v", err), http.StatusInternalServerError)
		return
	}

	// Search for transaction in all blocks
	for blockHeight, block := range chain.Blocks {
		for txIndex, tx := range block.Transactions {
			txHash := blockchain.HashTransaction(&tx)
			if txHash == targetHash {
				// Found transaction
				blockHash := blockchain.HashBlockHeader(&block.Header)
				response := map[string]interface{}{
					"found":       true,
					"transaction": tx,
					"block_height": blockHeight,
					"block_hash":   hex.EncodeToString(blockHash[:]),
					"tx_index":     txIndex,
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}
	}

	// Check mempool for pending transaction
	transactions := n.Mempool.GetTransactions(1000) // Get all transactions
	for _, tx := range transactions {
		txHash := blockchain.HashTransaction(&tx)
		if txHash == targetHash {
			// Found in mempool
			response := map[string]interface{}{
				"found":       true,
				"transaction": tx,
				"status":      "pending",
				"in_mempool":  true,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Transaction not found
	response := map[string]interface{}{
		"found": false,
		"hash":  path,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GET /block/<hash> - Get block information
func (n *FullNode) handleBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract hash from URL path
	path := strings.TrimPrefix(r.URL.Path, "/block/")
	if path == "" {
		http.Error(w, "Block hash required", http.StatusBadRequest)
		return
	}

	// Parse hash (support both hex and base64)
	var targetHash blockchain.Hash32
	var err error

	// Try hex first (more common for blockchain hashes)
	hashBytes, err := hex.DecodeString(path)
	if err != nil {
		// Try base64
		hashBytes, err = base64.StdEncoding.DecodeString(path)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid hash format (tried hex and base64): %v", err), http.StatusBadRequest)
			return
		}
	}

	if len(hashBytes) != 32 {
		http.Error(w, fmt.Sprintf("Hash must be 32 bytes, got %d bytes from '%s'", len(hashBytes), path), http.StatusBadRequest)
		return
	}

	copy(targetHash[:], hashBytes)

	// Get current chain
	chain, err := n.Store.GetChain()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get chain: %v", err), http.StatusInternalServerError)
		return
	}

	// Search for block
	for blockHeight, block := range chain.Blocks {
		blockHash := blockchain.HashBlockHeader(&block.Header)
		if blockHash == targetHash {
			// Found block
			response := map[string]interface{}{
				"found":         true,
				"block":         block,
				"height":        blockHeight,
				"hash":          hex.EncodeToString(blockHash[:]),
				"tx_count":      len(block.Transactions),
				"confirmations": len(chain.Blocks) - blockHeight,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Block not found
	response := map[string]interface{}{
		"found": false,
		"hash":  path,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}