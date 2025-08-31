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

	// Orphan block pool for handling blocks whose parents are not yet available
	orphanPool map[blockchain.Hash32]*blockchain.Block

	// Components (each package handles its own concern)
	httpServer *api.Server    // HTTP API server
	p2pServer  *p2p.Server    // P2P message handling
	discovery  *p2p.Discovery // Peer discovery
}

// NewFullNode creates a node that runs all services
func NewFullNode(config Config) *FullNode {
	// Create shared store
	chainStore := store.NewMemoryChainStore()

	return &FullNode{
		store:      chainStore,
		config:     config,
		orphanPool: make(map[blockchain.Hash32]*blockchain.Block),
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
		Port:           n.config.P2PPort,
		NodeID:         n.config.NodeID,
		Store:          n.store,
		BlockProcessor: n, // Pass the FullNode itself as the BlockProcessor
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

// Stop gracefully shuts down the FullNode
func (n *FullNode) Stop() error {
	log.Println("Stopping FullNode...")
	
	// Stop P2P server
	if n.p2pServer != nil {
		if err := n.p2pServer.Stop(); err != nil {
			log.Printf("Error stopping P2P server: %v", err)
		}
	}
	
	// Stop discovery
	if n.discovery != nil {
		// Discovery doesn't have a Stop method, but stopping P2P server should be enough
		log.Println("Discovery stopped")
	}
	
	// HTTP server doesn't have a graceful shutdown implemented yet @TODO
	
	log.Println("FullNode stopped successfully")
	return nil
}

// ProcessBlock attempts to add a block to the main chain, handling orphans
func (n *FullNode) ProcessBlock(block *blockchain.Block) error {
	blockHash := blockchain.HashBlockHeader(&block.Header)
	
	// Get current chain for validation
	chain, err := n.store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get chain: %w", err)
	}
	
	// Try to validate and add block directly to main chain
	if err := blockchain.ValidateAndApplyBlock(block, chain); err != nil {
		// Check if this is a missing parent error (orphan block)
		if missingParentErr, ok := err.(blockchain.ErrMissingParent); ok {
			log.Printf("Block %x is orphan, missing parent %x. Adding to orphan pool.", 
				blockHash[:8], missingParentErr.Hash[:8])
			
			// Store in orphan pool
			n.orphanPool[blockHash] = block
			
			// TODO: Request missing parent block from peers
			log.Printf("Need to request parent block %x from peers", missingParentErr.Hash[:8])
			return nil
		}
		
		// Other validation errors
		return fmt.Errorf("block validation failed: %w", err)
	}
	
	// Block validation succeeded, now persist it to the store
	if err := n.store.AddBlock(block); err != nil {
		return fmt.Errorf("failed to persist block to store: %w", err)
	}
	
	// Block successfully added to main chain
	log.Printf("Block %x added to main chain", blockHash[:8])
	
	// Relay the block to connected peers (if we have a P2P server)
	if n.p2pServer != nil {
		// Use empty string as excludePeerAddr since this block was added locally
		n.relayBlockToPeers(block)
	}
	
	// Try to connect any orphan blocks that might now be connectible
	n.tryConnectOrphans()
	
	return nil
}

// tryConnectOrphans attempts to connect orphan blocks to the main chain
func (n *FullNode) tryConnectOrphans() {
	connected := true
	
	// Keep trying until no more orphans can be connected
	for connected {
		connected = false
		
		// Get current chain state for each attempt
		chain, err := n.store.GetChain()
		if err != nil {
			log.Printf("Failed to get chain for orphan connection: %v", err)
			return
		}
		
		// Check each orphan block
		for orphanHash, orphanBlock := range n.orphanPool {
			// Try to validate and add this orphan block
			if err := blockchain.ValidateAndApplyBlock(orphanBlock, chain); err == nil {
				// Successfully connected!
				log.Printf("Connected orphan block %x to main chain", orphanHash[:8])
				
				// Remove from orphan pool
				delete(n.orphanPool, orphanHash)
				connected = true
				
				// Don't continue iterating as we modified the map
				break
			}
		}
	}
	
	if len(n.orphanPool) > 0 {
		log.Printf("Still have %d orphan blocks waiting for parents", len(n.orphanPool))
	}
}

// copyChainForValidation creates a deep copy of the chain for validation experiments
func (n *FullNode) copyChainForValidation() (*blockchain.Chain, error) {
	originalChain, err := n.store.GetChain()
	if err != nil {
		return nil, fmt.Errorf("failed to get original chain: %w", err)
	}
	
	// Create a deep copy
	chainCopy := &blockchain.Chain{
		Blocks:        make([]*blockchain.Block, len(originalChain.Blocks)),
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}
	
	// Copy blocks
	for i, block := range originalChain.Blocks {
		blockCopy := &blockchain.Block{
			Header:       block.Header,
			Transactions: make([]blockchain.Transaction, len(block.Transactions)),
		}
		copy(blockCopy.Transactions, block.Transactions)
		chainCopy.Blocks[i] = blockCopy
	}
	
	// Copy account states
	for pubkey, accountState := range originalChain.AccountStates {
		chainCopy.AccountStates[pubkey] = &blockchain.AccountState{
			Address: accountState.Address,
			Balance: accountState.Balance,
			Nonce:   accountState.Nonce,
		}
	}
	
	return chainCopy, nil
}

// relayBlockToPeers relays a block to all connected peers
func (n *FullNode) relayBlockToPeers(block *blockchain.Block) {
	// Delegate to the P2P server's relay method with empty excludePeerAddr (locally added block)
	if n.p2pServer != nil {
		// Access the relayBlockToOthers method through a public interface
		// We'll need to add a public method to the P2P server for this
		n.p2pServer.RelayBlock(block)
	}
}
