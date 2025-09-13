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
	Port   string
	NodeID    string
	SeedPeers []string
}

// FullNode orchestrates Peer Discovery and the rest of networking stuff
type FullNode struct {
	// Core blockchain storage
	store store.ChainStore

	// Configuration
	Config Config

	// Components (each package handles its own concern)
	NetworkServer *networking.Server    // Network message handling
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

		// 2. Start network server and wait for it to be ready
		networkReady := n.startNetworkingWithCompletion()
		if !<-networkReady {
			log.Printf("%s\tFailed to start network server", n.Config.NodeID)
			ready <- false
			return
		}

		// 3. Start peer discovery (network server is ready)
		discoveryReady := n.NetworkServer.StartDiscovery()
		if !<-discoveryReady {
			log.Printf("%s\tFailed to start discovery", n.Config.NodeID)
			ready <- false
			return
		}

		log.Printf("%s\tFull node started: Network on :%s", n.Config.NodeID, n.Config.Port)

		// Signal ready
		ready <- true

		// Block forever
		select {}
	}()

	return ready
}

func (n *FullNode) startNetworking() {
	log.Printf("%s\tStarting network server on port %s", n.Config.NodeID, n.Config.Port)
	// The networking package handles all network messaging
	networkConfig := networking.Config{
		Port:          n.Config.Port,
		NodeID:        n.Config.NodeID,
		Store:         n.store,
		SeedPeers:     n.Config.SeedPeers,
		ReqRespConfig: networking.DefaultReqRespConfig(), // Use default request-response configuration
	}
	n.NetworkServer = networking.NewServer(networkConfig)

	err := n.NetworkServer.Start()
	if err != nil {
		log.Printf("%s\tFailed to start network server: %v", n.Config.NodeID, err)
	}
}

// startNetworkingWithCompletion starts network server and signals when ready
func (n *FullNode) startNetworkingWithCompletion() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		log.Printf("%s\tStarting network server on port %s", n.Config.NodeID, n.Config.Port)

		networkConfig := networking.Config{
			Port:          n.Config.Port,
			NodeID:        n.Config.NodeID,
			Store:         n.store,
			SeedPeers:     n.Config.SeedPeers,
			ReqRespConfig: networking.DefaultReqRespConfig(),
		}
		n.NetworkServer = networking.NewServer(networkConfig)

		err := n.NetworkServer.Start()
		if err != nil {
			log.Printf("%s\tFailed to start network server: %v", n.Config.NodeID, err)
			ready <- false
		} else {
			ready <- true
		}
	}()

	return ready
}


// Stop gracefully shuts down the FullNode
func (n *FullNode) Stop() error {
	log.Printf("%s\tStopping FullNode...", n.Config.NodeID)

	// Stop network server
	if n.NetworkServer != nil {
		if err := n.NetworkServer.Stop(); err != nil {
			log.Printf("%s\tError stopping network server: %v", n.Config.NodeID, err)
		}
	}

	log.Println("FullNode stopped successfully")
	return nil
}

// GetNetworkServer returns the network server for testing purposes
func (n *FullNode) GetNetworkServer() *networking.Server {
	return n.NetworkServer
}

// GetNodeID returns the node's ID
func (n *FullNode) GetNodeID() string {
	return n.Config.NodeID
}

// SubmitTransaction submits a transaction to the network
func (n *FullNode) SubmitTransaction(tx *blockchain.Transaction) error {
	if n.NetworkServer == nil {
		return fmt.Errorf("network server not initialized")
	}

	// Broadcast the transaction to all connected peers
	go func() { <-networking.BroadcastTransaction(n.NetworkServer, tx) }()
	return nil
}
