package api

import (
	"gocuria/api/handlers"
	"gocuria/blockchain/store"
	"net/http"
	"log"
)

func StartServer(store store.ChainStore) {
	mux := http.NewServeMux()

	// Block endpoints
	mux.HandleFunc("/api/blocks", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleBlocks(w, r, store) 
	})
	mux.HandleFunc("/api/blocks/", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleBlocks(w, r, store) // Handles /api/blocks/{hash}
	})

	// Chain endpoints  
	mux.HandleFunc("/api/chain/height", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleChainHeight(w, r, store)
	})
	mux.HandleFunc("/api/chain/head", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleChainHead(w, r, store)
	})

	// Transaction endpoints
	mux.HandleFunc("/api/transactions", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleTransactions(w, r, store)
	})

	log.Println("Starting server on :8372")
	log.Fatal(http.ListenAndServe(":8372", mux))
}
