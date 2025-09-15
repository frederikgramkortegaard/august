package queryapi

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

// Node interface represents the minimal interface needed by the query API
type Node interface {
	GetChainHead() blockchain.ChainHead
	ProcessBlock(block *blockchain.Block, excludePeerAddr ...string) <-chan struct{}
	SubmitTransaction(tx *blockchain.Transaction) error
	GetChain() (*blockchain.Chain, error)
	GetMempool() interface{ GetTransactions(limit int) []blockchain.Transaction }
}

// StartQueryAPI starts the HTTP API server for miners and blockchain queries (completely separate from P2P)
func StartQueryAPI(node Node, port int) error {
	// Create a new ServeMux to avoid conflicts when running multiple instances
	mux := http.NewServeMux()

	// Miner and transaction endpoints
	mux.HandleFunc("/chain-info", handleChainInfo(node))
	mux.HandleFunc("/chain-state", handleChainState(node))
	mux.HandleFunc("/submit-block", handleSubmitBlock(node))
	mux.HandleFunc("/submit-transaction", handleSubmitTransaction(node))
	mux.HandleFunc("/mempool", handleGetMempool(node))

	// Query API endpoints for blockchain explorer functionality
	mux.HandleFunc("/balance/", handleBalance(node))
	mux.HandleFunc("/transaction/", handleTransaction(node))
	mux.HandleFunc("/block/", handleBlock(node))
	mux.HandleFunc("/contract/", handleContract(node))

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting Query API server on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

// GET /chain-info - Returns the current chain head info
func handleChainInfo(node Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		chainHead := node.GetChainHead()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chainHead)
	}
}

// GET /chain-state - Returns the current account states for miners
func handleChainState(node Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get current chain state
		chain, err := node.GetChain()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get chain state: %v", err), http.StatusInternalServerError)
			return
		}

		// Convert map keys from PublicKey to hex string for JSON serialization
		accountStatesJSON := make(map[string]*blockchain.AccountState)
		for pubKey, state := range chain.AccountStates {
			keyHex := hex.EncodeToString(pubKey[:])
			accountStatesJSON[keyHex] = state
		}

		response := ChainStateResponse{
			AccountStates: accountStatesJSON,
			AccountCount:  len(chain.AccountStates),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// POST /submit-block - Submit a mined block
func handleSubmitBlock(node Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			<-node.ProcessBlock(&block)
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "submitted"})
	}
}

// GET /mempool - Returns pending transactions from mempool (highest fee first)
func handleGetMempool(node Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		transactions := node.GetMempool().GetTransactions(limit)

		response := MempoolResponse{
			Count:        len(transactions),
			Transactions: transactions,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GET /balance/<address> - Check account balance
func handleBalance(node Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		chain, err := node.GetChain()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get chain state: %v", err), http.StatusInternalServerError)
			return
		}

		// Look up account state
		accountState, exists := chain.AccountStates[publicKey]

		response := BalanceResponse{
			Address: path,
			Exists:  exists,
		}

		if exists {
			response.Balance = accountState.Balance
			response.Nonce = accountState.Nonce
		} else {
			response.Balance = 0
			response.Nonce = 0
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GET /transaction/<hash> - Lookup transaction details
func handleTransaction(node Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		chain, err := node.GetChain()
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
					response := TransactionResponse{
						Found:       true,
						Transaction: tx,
						BlockHeight: blockHeight,
						BlockHash:   hex.EncodeToString(blockHash[:]),
						TxIndex:     txIndex,
					}

					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(response)
					return
				}
			}
		}

		// Check mempool for pending transaction
		transactions := node.GetMempool().GetTransactions(1000) // Get all transactions
		for _, tx := range transactions {
			txHash := blockchain.HashTransaction(&tx)
			if txHash == targetHash {
				// Found in mempool
				response := TransactionResponse{
					Found:       true,
					Transaction: tx,
					Status:      "pending",
					InMempool:   true,
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}

		// Transaction not found
		response := TransactionResponse{
			Found: false,
			Hash:  path,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GET /block/<hash> - Get block information
func handleBlock(node Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		chain, err := node.GetChain()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get chain: %v", err), http.StatusInternalServerError)
			return
		}

		// Search for block
		for blockHeight, block := range chain.Blocks {
			blockHash := blockchain.HashBlockHeader(&block.Header)
			if blockHash == targetHash {
				// Found block
				response := BlockResponse{
					Found:         true,
					Block:         block,
					Height:        blockHeight,
					Hash:          hex.EncodeToString(blockHash[:]),
					TxCount:       len(block.Transactions),
					Confirmations: len(chain.Blocks) - blockHeight,
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}

		// Block not found
		response := BlockResponse{
			Found: false,
			Hash:  path,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// POST /submit-transaction - Submit a transaction to the mempool
func handleSubmitTransaction(node Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var tx blockchain.Transaction
		if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
			http.Error(w, fmt.Sprintf("Invalid transaction format: %v", err), http.StatusBadRequest)
			return
		}

		// Submit the transaction (this validates and adds to mempool + relays to peers)
		if err := node.SubmitTransaction(&tx); err != nil {
			http.Error(w, fmt.Sprintf("Transaction rejected: %v", err), http.StatusBadRequest)
			return
		}

		// Return success response with transaction hash
		txHash := blockchain.HashTransaction(&tx)
		response := SubmitTransactionResponse{
			Status: "submitted",
			Hash:   hex.EncodeToString(txHash[:]),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GET /contract/<address> - Get contract information and persistent storage
func handleContract(node Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract address from URL path
		path := strings.TrimPrefix(r.URL.Path, "/contract/")
		if path == "" {
			http.Error(w, "Contract address required", http.StatusBadRequest)
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
		chain, err := node.GetChain()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get chain state: %v", err), http.StatusInternalServerError)
			return
		}

		// Look up account state
		accountState, exists := chain.AccountStates[publicKey]

		response := ContractResponse{
			Found:   exists,
			Address: path,
		}

		if exists {
			response.Balance = accountState.Balance
			response.Nonce = accountState.Nonce
			response.IsContract = len(accountState.Instructions) > 0
			response.HasInstructions = len(accountState.Instructions) > 0
			response.StorageEntries = len(accountState.Persistent)

			// Include persistent storage data if it exists
			if len(accountState.Persistent) > 0 {
				response.PersistentData = make(map[string]string)
				for k, v := range accountState.Persistent {
					response.PersistentData[k] = v
				}
			}

			// Add code hash and storage root if available
			response.CodeHash = hex.EncodeToString(accountState.CodeHash[:])
			response.StorageRoot = hex.EncodeToString(accountState.StorageRoot[:])
		} else {
			response.IsContract = false
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}