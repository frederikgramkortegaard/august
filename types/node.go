package types

import (
	"context"
	"sync"
)

// Config holds all configuration for a full node
type NodeConfig struct {
	Port         string
	NodeID       string
	SeedPeers    []string
	DatabaseName string
	QueryPort    string // HTTP port for query API and miners (optional)
}

// FullNode orchestrates Peer Discovery and the rest of networking stuff
type FullNode struct {

	// Configuration
	Config NodeConfig

	// Components (each package handles its own concern)
	NetworkServer *Network          // Network message handling
	Mempool       interface{}  // Transaction pool
	ChainStorage  interface{} // Blockchain Interactions

	// Goroutine management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}
