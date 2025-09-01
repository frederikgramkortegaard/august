package node

import (
	"fmt"
	"gocuria/blockchain"
	"gocuria/blockchain/processing"
	"gocuria/blockchain/store"
	"gocuria/p2p"
	"log"
	"time"
)

// Config holds all configuration for a full node
type Config struct {
	P2PPort   string
	NodeID    string
	SeedPeers []string
}

// FullNode orchestrates Peer Discovery and the rest of P2P stuff
type FullNode struct {
	// Core blockchain storage
	store store.ChainStore

	// Configuration
	config Config

	// Block processing (handles validation, orphans, etc.)
	// We have this because BlockProcessing will also make P2P requests
	// e.g. to request missing blocks
	blockProcessor *processing.BlockProcessor

	// Components (each package handles its own concern)
	p2pServer *p2p.Server    // P2P message handling
	discovery *p2p.Discovery // Peer discovery
}

// NewFullNode creates a node that runs all services
func NewFullNode(config Config) *FullNode {
	// Create shared store
	chainStore := store.NewMemoryChainStore()

	// Create block processor
	blockProcessor := processing.NewBlockProcessor(chainStore)

	return &FullNode{
		store:          chainStore,
		config:         config,
		blockProcessor: blockProcessor,
		// Components will be initialized in Start()
	}
}

// Start initializes and starts all components
func (n *FullNode) Start() error {
	// 1. Initialize blockchain with genesis
	if err := n.store.AddBlock(blockchain.GenesisBlock); err != nil {
		return fmt.Errorf("failed to add genesis block: %w", err)
	}
	log.Printf("%s\tBlockchain initialized with genesis block", n.config.NodeID)

	//  Start P2P server (for node-to-node communication)
	go n.startP2P()

	// 4. Start peer discovery (find and connect to other nodes)
	// Wait a moment for P2P server to be ready
	time.Sleep(100 * time.Millisecond)
	go n.startDiscovery()

	log.Printf("%s\tFull node started: P2P on :%s", n.config.NodeID, n.config.P2PPort)

	// Block forever
	select {}
}

func (n *FullNode) startP2P() {
	log.Printf("%s\tStarting P2P server on port %s", n.config.NodeID, n.config.P2PPort)
	// The p2p package handles all P2P messaging
	p2pConfig := p2p.Config{
		Port:           n.config.P2PPort,
		NodeID:         n.config.NodeID,
		Store:          n.store,
		BlockProcessor: n.blockProcessor, // Use the dedicated block processor
	}
	n.p2pServer = p2p.NewServer(p2pConfig)

	// Link the block processor with the P2P server for relaying
	n.blockProcessor.SetP2PServer(n.p2pServer)

	err := n.p2pServer.Start()
	if err != nil {
		log.Printf("%s\tFailed to start P2P server: %v", n.config.NodeID, err)
	}
}

func (n *FullNode) startDiscovery() {
	log.Printf("%s\tStarting peer discovery...", n.config.NodeID)
	// The p2p package also handles discovery
	discoveryConfig := p2p.DiscoveryConfig{
		SeedPeers: n.config.SeedPeers,
		P2PServer: n.p2pServer,
	}
	n.discovery = p2p.NewDiscovery(discoveryConfig)
	n.discovery.Start()
}

// Stop gracefully shuts down the FullNode
func (n *FullNode) Stop() error {
	log.Printf("%s\tStopping FullNode...", n.config.NodeID)

	// Stop P2P server
	if n.p2pServer != nil {
		if err := n.p2pServer.Stop(); err != nil {
			log.Printf("%s\tError stopping P2P server: %v", n.config.NodeID, err)
		}
	}

	// Stop discovery
	if n.discovery != nil {
		// Discovery doesn't have a Stop method, but stopping P2P server should be enough
		log.Println("Discovery stopped")
	}

	log.Println("FullNode stopped successfully")
	return nil
}

// GetP2PServer returns the P2P server for testing purposes
func (n *FullNode) GetP2PServer() *p2p.Server {
	return n.p2pServer
}

// GetDiscovery returns the discovery instance for testing purposes
func (n *FullNode) GetDiscovery() *p2p.Discovery {
	return n.discovery
}
