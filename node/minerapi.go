package node

import (
	"august/blockchain"
	"encoding/json"
	"fmt"
	"net/http"
)

// StartMinerAPI starts the HTTP API server for miners (completely separate from P2P)
func (n *FullNode) StartMinerAPI(port int) error {
	http.HandleFunc("/chain-info", n.handleChainInfo)
	http.HandleFunc("/submit-block", n.handleSubmitBlock)

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