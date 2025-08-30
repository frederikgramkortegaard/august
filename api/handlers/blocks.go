package handlers

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"net/http"
	"strings"
)

func HandleBlocks(w http.ResponseWriter, r *http.Request, store store.ChainStore) {

	switch r.Method {
	case http.MethodPost:
		handlePostBlock(w, r, store)
	case http.MethodGet:
		handleGetBlockByHash(w, r, store)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}

}

func handlePostBlock(w http.ResponseWriter, r *http.Request, store store.ChainStore) {

	// 1. Deserialize
	var block blockchain.Block
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 2. Business Logic
	if err := store.AddBlock(&block); err != nil {
		http.Error(w, fmt.Sprintf("Block validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// 3. Success Response
	hash := blockchain.HashBlockHeader(&block.Header)
	response := map[string]string{
		"status":  "success",
		"hash":    fmt.Sprintf("%x", hash),
		"message": "Block added to chain",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func handleGetBlockByHash(w http.ResponseWriter, r *http.Request, store store.ChainStore) {
	// Extract hash from URL path: /api/blocks/{hash}
	path := strings.TrimPrefix(r.URL.Path, "/api/blocks/")
	if path == "" || path == "/api/blocks" {
		http.Error(w, "Block hash required in URL", http.StatusBadRequest)
		return
	}

	// Convert hex string to Hash32
	hashBytes, err := hex.DecodeString(path)
	if err != nil || len(hashBytes) != 32 {
		http.Error(w, "Invalid block hash format (must be 64 hex characters)", http.StatusBadRequest)
		return
	}

	var hash blockchain.Hash32
	copy(hash[:], hashBytes)

	// Get block from store
	block, err := store.GetBlockByHash(hash)
	if err != nil {
		http.Error(w, fmt.Sprintf("Block not found: %v", err), http.StatusNotFound)
		return
	}

	// Return block as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(block)
}
