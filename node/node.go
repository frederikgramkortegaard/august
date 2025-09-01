package node

import (
	"fmt"
	"gocuria/blockchain"
	"gocuria/blockchain/processing"
	"gocuria/blockchain/store"
	"gocuria/p2p"
	"log"
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

// Start initializes and starts all components and returns when ready
func (n *FullNode) Start() <-chan bool {
	ready := make(chan bool, 1)
	
	go func() {
		// 1. Initialize blockchain with genesis
		if err := n.store.AddBlock(blockchain.GenesisBlock); err != nil {
			log.Printf("%s\tFailed to add genesis block: %v", n.config.NodeID, err)
			ready <- false
			return
		}
		log.Printf("%s\tBlockchain initialized with genesis block", n.config.NodeID)

		// 2. Start P2P server and wait for it to be ready
		p2pReady := n.startP2PWithCompletion()
		if !<-p2pReady {
			log.Printf("%s\tFailed to start P2P server", n.config.NodeID)
			ready <- false
			return
		}
		
		// 3. Start peer discovery (P2P server is ready)
		discoveryReady := n.startDiscovery()
		if !<-discoveryReady {
			log.Printf("%s\tFailed to start discovery", n.config.NodeID)
			ready <- false
			return
		}
		
		log.Printf("%s\tFull node started: P2P on :%s", n.config.NodeID, n.config.P2PPort)
		
		// Signal ready
		ready <- true
		
		// Block forever
		select {}
	}()
	
	return ready
}

func (n *FullNode) startP2P() {
	log.Printf("%s\tStarting P2P server on port %s", n.config.NodeID, n.config.P2PPort)
	// The p2p package handles all P2P messaging
	p2pConfig := p2p.Config{
		Port:           n.config.P2PPort,
		NodeID:         n.config.NodeID,
		Store:          n.store,
		BlockProcessor: n.blockProcessor, // Use the dedicated block processor
		ReqRespConfig:  p2p.DefaultReqRespConfig(), // Use default request-response configuration
	}
	n.p2pServer = p2p.NewServer(p2pConfig)

	// Link the block processor with the P2P server for relaying
	n.blockProcessor.SetP2PServer(n.p2pServer)

	err := n.p2pServer.Start()
	if err != nil {
		log.Printf("%s\tFailed to start P2P server: %v", n.config.NodeID, err)
	}
}

// startP2PWithCompletion starts P2P server and signals when ready
func (n *FullNode) startP2PWithCompletion() <-chan bool {
	ready := make(chan bool, 1)
	
	go func() {
		log.Printf("%s\tStarting P2P server on port %s", n.config.NodeID, n.config.P2PPort)
		
		p2pConfig := p2p.Config{
			Port:           n.config.P2PPort,
			NodeID:         n.config.NodeID,
			Store:          n.store,
			BlockProcessor: n.blockProcessor,
			ReqRespConfig:  p2p.DefaultReqRespConfig(),
		}
		n.p2pServer = p2p.NewServer(p2pConfig)
		n.blockProcessor.SetP2PServer(n.p2pServer)

		err := n.p2pServer.Start()
		if err != nil {
			log.Printf("%s\tFailed to start P2P server: %v", n.config.NodeID, err)
			ready <- false
		} else {
			ready <- true
		}
	}()
	
	return ready
}

func (n *FullNode) startDiscovery() <-chan bool {
	ready := make(chan bool, 1)
	
	go func() {
		log.Printf("%s\tStarting peer discovery...", n.config.NodeID)
		
		// Double-check P2P server is available
		if n.p2pServer == nil {
			log.Printf("%s\tP2P server not ready for discovery", n.config.NodeID)
			ready <- false
			return
		}
		
		// The p2p package also handles discovery
		discoveryConfig := p2p.DiscoveryConfig{
			SeedPeers: n.config.SeedPeers,
			P2PServer: n.p2pServer,
		}
		n.discovery = p2p.NewDiscovery(discoveryConfig)
		discoveryStartReady := n.discovery.Start()
		if !<-discoveryStartReady {
			log.Printf("%s\tDiscovery failed to start", n.config.NodeID)
			ready <- false
			return
		}
		
		ready <- true
	}()
	
	return ready
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

// SubmitTransaction submits a transaction to the network
func (n *FullNode) SubmitTransaction(tx *blockchain.Transaction) error {
	if n.p2pServer == nil {
		return fmt.Errorf("P2P server not initialized")
	}
	
	// Broadcast the transaction to all connected peers
	return n.p2pServer.BroadcastTransaction(tx)
}
