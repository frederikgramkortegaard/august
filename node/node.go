package node

import (
	"august/blockchain"
	"august/networking"
	store "august/storage"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
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
	Config Config

	// Components (each package handles its own concern)
	P2PServer *networking.Server    // P2P message handling
	discovery *networking.Discovery // Peer discovery
}

// NewFullNode creates a node that runs all services
func NewFullNode(config Config) *FullNode {
	// Create shared store
	chainStore := store.NewPebbleChainStore("demo", &pebble.Options{})

	return &FullNode{
		store:  chainStore,
		Config: config,
		// Components will be initialized in Start()
	}
}

// Start initializes and starts all components and returns when ready
func (n *FullNode) Start() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		// 1. Initialize blockchain with genesis
		if err := n.store.AddBlock(blockchain.GenesisBlock); err != nil {
			log.Printf("%s\tFailed to add genesis block: %v", n.Config.NodeID, err)
			ready <- false
			return
		}
		log.Printf("%s\tBlockchain initialized with genesis block", n.Config.NodeID)

		// 2. Start P2P server and wait for it to be ready
		p2pReady := n.startP2PWithCompletion()
		if !<-p2pReady {
			log.Printf("%s\tFailed to start P2P server", n.Config.NodeID)
			ready <- false
			return
		}

		// 3. Start peer discovery (P2P server is ready)
		discoveryReady := n.startDiscovery()
		if !<-discoveryReady {
			log.Printf("%s\tFailed to start discovery", n.Config.NodeID)
			ready <- false
			return
		}

		log.Printf("%s\tFull node started: P2P on :%s", n.Config.NodeID, n.Config.P2PPort)

		// Signal ready
		ready <- true

		// Block forever
		select {}
	}()

	return ready
}

func (n *FullNode) startP2P() {
	log.Printf("%s\tStarting P2P server on port %s", n.Config.NodeID, n.Config.P2PPort)
	// The p2p package handles all P2P messaging
	p2pConfig := networking.Config{
		Port:          n.Config.P2PPort,
		NodeID:        n.Config.NodeID,
		Store:         n.store,
		ReqRespConfig: networking.DefaultReqRespConfig(), // Use default request-response configuration
	}
	n.P2PServer = networking.NewServer(p2pConfig)

	err := n.P2PServer.Start()
	if err != nil {
		log.Printf("%s\tFailed to start P2P server: %v", n.Config.NodeID, err)
	}
}

// startP2PWithCompletion starts P2P server and signals when ready
func (n *FullNode) startP2PWithCompletion() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		log.Printf("%s\tStarting P2P server on port %s", n.Config.NodeID, n.Config.P2PPort)

		p2pConfig := networking.Config{
			Port:          n.Config.P2PPort,
			NodeID:        n.Config.NodeID,
			Store:         n.store,
			ReqRespConfig: networking.DefaultReqRespConfig(),
		}
		n.P2PServer = networking.NewServer(p2pConfig)

		err := n.P2PServer.Start()
		if err != nil {
			log.Printf("%s\tFailed to start P2P server: %v", n.Config.NodeID, err)
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
		log.Printf("%s\tStarting peer discovery...", n.Config.NodeID)

		// Double-check P2P server is available
		if n.P2PServer == nil {
			log.Printf("%s\tP2P server not ready for discovery", n.Config.NodeID)
			ready <- false
			return
		}

		// The p2p package also handles discovery
		discoveryConfig := networking.DiscoveryConfig{
			SeedPeers: n.Config.SeedPeers,
			P2PServer: n.P2PServer,
		}
		n.discovery = networking.NewDiscovery(discoveryConfig)
		discoveryStartReady := n.discovery.Start()
		if !<-discoveryStartReady {
			log.Printf("%s\tDiscovery failed to start", n.Config.NodeID)
			ready <- false
			return
		}

		ready <- true
	}()

	return ready
}

// Stop gracefully shuts down the FullNode
func (n *FullNode) Stop() error {
	log.Printf("%s\tStopping FullNode...", n.Config.NodeID)

	// Stop P2P server
	if n.P2PServer != nil {
		if err := n.P2PServer.Stop(); err != nil {
			log.Printf("%s\tError stopping P2P server: %v", n.Config.NodeID, err)
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
func (n *FullNode) GetP2PServer() *networking.Server {
	return n.P2PServer
}

// GetDiscovery returns the discovery instance for testing purposes
func (n *FullNode) GetDiscovery() *networking.Discovery {
	return n.discovery
}

// GetNodeID returns the node's ID
func (n *FullNode) GetNodeID() string {
	return n.Config.NodeID
}

// SubmitTransaction submits a transaction to the network
func (n *FullNode) SubmitTransaction(tx *blockchain.Transaction) error {
	if n.P2PServer == nil {
		return fmt.Errorf("P2P server not initialized")
	}

	// Broadcast the transaction to all connected peers
	go func() { <-networking.BroadcastTransaction(n.P2PServer, tx) }()
	return nil
}
