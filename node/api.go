package node

import (
	"august/blockchain"
	"august/networking"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// API Response Types

// BalanceResponse represents the response from the balance API endpoint
type BalanceResponse struct {
	Address string `json:"address"`
	Exists  bool   `json:"exists"`
	Balance uint64 `json:"balance"`
	Nonce   uint64 `json:"nonce"`
}

// SubmitTransactionResponse represents the response from submitting a transaction
type SubmitTransactionResponse struct {
	Status string `json:"status"`
	Hash   string `json:"hash"`
}

// TransactionResponse represents the response from the transaction lookup API
type TransactionResponse struct {
	Found       bool        `json:"found"`
	Transaction interface{} `json:"transaction,omitempty"`
	BlockHeight int         `json:"block_height,omitempty"`
	BlockHash   string      `json:"block_hash,omitempty"`
	TxIndex     int         `json:"tx_index,omitempty"`
	Status      string      `json:"status,omitempty"`
	InMempool   bool        `json:"in_mempool,omitempty"`
	Hash        string      `json:"hash,omitempty"`
}

// BlockResponse represents the response from the block lookup API
type BlockResponse struct {
	Found         bool        `json:"found"`
	Block         interface{} `json:"block,omitempty"`
	Height        int         `json:"height,omitempty"`
	Hash          string      `json:"hash,omitempty"`
	TxCount       int         `json:"tx_count,omitempty"`
	Confirmations int         `json:"confirmations,omitempty"`
}

// MempoolResponse represents the response from the mempool API
type MempoolResponse struct {
	Count        int                      `json:"count"`
	Transactions []blockchain.Transaction `json:"transactions"`
}

// ContractResponse represents the response from the contract inspection API
type ContractResponse struct {
	Found           bool              `json:"found"`
	Address         string            `json:"address"`
	Balance         uint64            `json:"balance,omitempty"`
	Nonce           uint64            `json:"nonce,omitempty"`
	IsContract      bool              `json:"is_contract"`
	HasInstructions bool              `json:"has_instructions,omitempty"`
	StorageEntries  int               `json:"storage_entries,omitempty"`
	PersistentData  map[string]string `json:"persistent_data,omitempty"`
	CodeHash        string            `json:"code_hash,omitempty"`
	StorageRoot     string            `json:"storage_root,omitempty"`
}

// ChainStateResponse represents the response from the chain state API
type ChainStateResponse struct {
	AccountStates map[string]*blockchain.AccountState `json:"account_states"`
	AccountCount  int                                 `json:"account_count"`
}

// API Implementation

// NodeAPI interface represents the minimal interface needed by the query API
type NodeAPI interface {
	GetChainHead() blockchain.ChainHead
	ProcessHeader(header *blockchain.BlockHeader, sourcePeer string) error
	ProcessTransaction(tx *blockchain.Transaction) error
	GetChain() (*blockchain.Chain, error)
	GetMempool() *Mempool
	StoreBlock(block *blockchain.Block) error
}

// StartQueryAPI starts the HTTP API server for miners and blockchain queries (completely separate from P2P)
func StartQueryAPI(node NodeAPI, port int) error {
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
func handleChainInfo(node NodeAPI) http.HandlerFunc {
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
func handleChainState(node NodeAPI) http.HandlerFunc {
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
func handleSubmitBlock(node NodeAPI) http.HandlerFunc {
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

		// Store the block first, then process the header asynchronously
		go func() {
			// Store the block so it's available when ProcessHeader tries to fetch it
			err := node.StoreBlock(&block)
			if err != nil {
				log.Printf("Failed to store block from API: %v", err)
				return
			}

			// Now process the header
			err = node.ProcessHeader(&block.Header, "api")
			if err != nil {
				log.Printf("Failed to process header from API: %v", err)
				return
			}

			// For API-submitted blocks (like from miners), propagate to peers
			// Cast to access NetworkServer (this is safe since we know the implementation)
			if nodeImpl, ok := node.(*Node); ok {
				go func() { <-networking.RelayBlockHeader(nodeImpl.NetworkServer, &block.Header) }()
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "submitted"})
	}
}

// GET /mempool - Returns pending transactions from mempool (highest fee first)
func handleGetMempool(node NodeAPI) http.HandlerFunc {
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
func handleBalance(node NodeAPI) http.HandlerFunc {
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
func handleTransaction(node NodeAPI) http.HandlerFunc {
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
				txHash := tx.GetHash()
				if txHash == targetHash {
					// Found transaction
					blockHash := block.Header.GetHash()
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
			txHash := tx.GetHash()
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
func handleBlock(node NodeAPI) http.HandlerFunc {
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
			blockHash := block.Header.GetHash()
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
func handleSubmitTransaction(node NodeAPI) http.HandlerFunc {
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
		if err := node.ProcessTransaction(&tx); err != nil {
			http.Error(w, fmt.Sprintf("Transaction rejected: %v", err), http.StatusBadRequest)
			return
		}

		// Return success response with transaction hash
		txHash := tx.GetHash()
		response := SubmitTransactionResponse{
			Status: "submitted",
			Hash:   hex.EncodeToString(txHash[:]),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// GET /contract/<address> - Get contract information and persistent storage
func handleContract(node NodeAPI) http.HandlerFunc {
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
