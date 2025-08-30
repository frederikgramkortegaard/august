package main

import (
	"gocuria/api"
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"log"
)

func main() {
	// 1. Initialize the blockchain store
	chainStore := store.NewMemoryChainStore()

	// 2. Bootstrap with genesis block
	if err := chainStore.AddBlock(blockchain.GenesisBlock); err !=
		nil {
		log.Fatal("Failed to add genesis block:", err)
	}

	// 3. Start the HTTP API server
	log.Println("Starting blockchain node...")
	api.StartServer(chainStore) // This blocks forever
}
