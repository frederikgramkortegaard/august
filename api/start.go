package api

import (
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"log"
)

// StartSimpleHTTPNode starts a standalone HTTP-only node (no P2P)
// This is what cmd/node/main.go uses for simple testing
func StartSimpleHTTPNode(httpPort string) error {
	// Create store
	chainStore := store.NewMemoryChainStore()
	
	// Add genesis block
	if err := chainStore.AddBlock(blockchain.GenesisBlock); err != nil {
		return err
	}
	
	log.Println("Starting HTTP-only blockchain node...")
	
	// Start HTTP server (blocks forever)
	StartServer(chainStore, httpPort)
	
	return nil
}