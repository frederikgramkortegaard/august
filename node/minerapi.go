package node

import (
	"august/blockchain"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// StartMinerAPI starts the HTTP API server for miners (completely separate from P2P)
func (n *FullNode) StartMinerAPI(port int) error {
	http.HandleFunc("/chain-info", n.handleChainInfo)
	http.HandleFunc("/submit-block", n.handleSubmitBlock)
	http.HandleFunc("/mempool", n.handleGetMempool)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting miner HTTP API on %s\n", addr)
	return http.ListenAndServe(addr, nil)
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