package handlers

import (
	"encoding/json"
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"net/http"
)

func HandleBlocks(w http.ResponseWriter, r *http.Request, store store.ChainStore) {

	switch r.Method {
	case http.MethodPost:
		handlePostBlock(w, r, store)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 3. Success Response
	w.Header().Set("Content=Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})

}
