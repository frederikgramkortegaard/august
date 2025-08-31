package handlers

import (
	"encoding/json"
	"fmt"
	"gocuria/blockchain/store"
	"net/http"
)

func HandleChainHeight(w http.ResponseWriter, r *http.Request, store store.ChainStore) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	height, err := store.GetChainHeight()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get chain height: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]uint64{
		"height": height,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func HandleChainHead(w http.ResponseWriter, r *http.Request, store store.ChainStore) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	block, err := store.GetHeadBlock()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get head block: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(block)
}
