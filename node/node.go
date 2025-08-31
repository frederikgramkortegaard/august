package node

import (
	"fmt"
	"gocuria/api"
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"gocuria/p2p"
	"log"
	"time"
)

// Config holds all configuration for a full node
type Config struct {
	HTTPPort  string
	P2PPort   string
	NodeID    string
	SeedPeers []string
}

// FullNode orchestrates all components: HTTP API, P2P, Discovery
type FullNode struct {
	// Core blockchain storage
	store store.ChainStore
	
	// Configuration
	config Config
	
	// Components (each package handles its own concern)
	httpServer  *api.Server      // HTTP API server
	p2pServer   *p2p.Server      // P2P message handling
	discovery   *p2p.Discovery   // Peer discovery
}

// NewFullNode creates a node that runs all services
func NewFullNode(config Config) *FullNode {
	// Create shared store
	chainStore := store.NewMemoryChainStore()
	
	return &FullNode{
		store:  chainStore,
		config: config,
		// Components will be initialized in Start()
	}
}

// Start initializes and starts all components
func (n *FullNode) Start() error {
	// 1. Initialize blockchain with genesis
	if err := n.store.AddBlock(blockchain.GenesisBlock); err != nil {
		return fmt.Errorf("failed to add genesis block: %w", err)
	}
	log.Println("Blockchain initialized with genesis block")
	
	// 2. Start HTTP API server (for wallets/clients)
	go n.startHTTPAPI()
	
	// 3. Start P2P server (for node-to-node communication)
	go n.startP2P()
	
	// 4. Start peer discovery (find and connect to other nodes)
	// Wait a moment for P2P server to be ready
	time.Sleep(100 * time.Millisecond)
	go n.startDiscovery()
	
	log.Printf("Full node started: HTTP on :%s, P2P on :%s", 
		n.config.HTTPPort, n.config.P2PPort)
	
	// Block forever
	select {}
}

func (n *FullNode) startHTTPAPI() {
	log.Printf("Starting HTTP API on port %s", n.config.HTTPPort)
	n.httpServer = api.NewServer(n.store, n.config.HTTPPort)
	if err := n.httpServer.Start(); err != nil {
		log.Printf("HTTP server error: %v", err)
	}
}

func (n *FullNode) startP2P() {
	log.Printf("Starting P2P server on port %s", n.config.P2PPort)
	// The p2p package handles all P2P messaging
	p2pConfig := p2p.Config{
		Port:   n.config.P2PPort,
		NodeID: n.config.NodeID,
		Store:  n.store,
	}
	n.p2pServer = p2p.NewServer(p2pConfig)
	err := n.p2pServer.Start()
	if err != nil {
		log.Printf("Failed to start P2P server: %v", err)
	}
}

func (n *FullNode) startDiscovery() {
	log.Println("Starting peer discovery...")
	// The p2p package also handles discovery
	discoveryConfig := p2p.DiscoveryConfig{
		SeedPeers: n.config.SeedPeers,
		P2PServer: n.p2pServer,
	}
	n.discovery = p2p.NewDiscovery(discoveryConfig)
	n.discovery.Start()
}