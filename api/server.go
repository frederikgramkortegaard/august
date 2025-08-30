package api

import (
	"gocuria/api/handlers"
	"gocuria/blockchain/store"
	"net/http"
	"log"
)

func StartServer(store store.ChainStore) {
	mux := http.NewServeMux()

	// Register Routes
	 // Create handlers with store access
	 mux.HandleFunc("/api/blocks", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleBlocks(w, r, store) 
	})

	log.Println("Starting server on :8372")
	log.Fatal(http.ListenAndServe(":8372", mux))
}
