package handlers

import (
	"encoding/json"
	"fmt"
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"log"
	"net/http"
)

func HandleTransactions(w http.ResponseWriter, r *http.Request, store store.ChainStore) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	handleValidateTransaction(w, r, store)
}

func handleValidateTransaction(w http.ResponseWriter, r *http.Request, store store.ChainStore) {
	log.Println("Received transaction validation request")

	// Deserialize JSON from request body
	var transaction blockchain.Transaction
	if err := json.NewDecoder(r.Body).Decode(&transaction); err != nil {
		log.Printf("Failed to decode JSON: %v", err)
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	log.Printf("Decoded transaction: from=%x, to=%x, amount=%d, nonce=%d",
		transaction.From[:4], transaction.To[:4], transaction.Amount, transaction.Nonce)

	// Get account states for validation
	accountStates, err := store.GetAccountStates()
	if err != nil {
		log.Printf("Failed to get account states: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	// Validate transaction (read-only)
	log.Println("Validating transaction")
	if err := blockchain.ValidateTransaction(&transaction, accountStates); err != nil {
		log.Printf("Transaction validation failed: %v", err)
		response := map[string]string{
			"status": "invalid",
			"error":  err.Error(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Success response
	log.Println("Transaction validation successful")
	hash := blockchain.HashTransaction(&transaction)
	response := map[string]string{
		"status":  "valid",
		"hash":    fmt.Sprintf("%x", hash),
		"message": "Transaction is valid",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	log.Println("Response sent successfully")
}
