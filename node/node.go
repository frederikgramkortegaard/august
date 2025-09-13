package node

import (
	"august/blockchain"
	"august/networking"
	store "august/storage"
	"fmt"
	"log"
)

// Config holds all configuration for a full node
type Config struct {
	Port      string
	NodeID    string
	SeedPeers []string
	DBName    string
}

// FullNode orchestrates Peer Discovery and the rest of networking stuff
type FullNode struct {
	// Core blockchain storage
	Store store.ChainStore

	// Configuration
	Config Config

	// Components (each package handles its own concern)
	NetworkServer *networking.Server // Network message handling
}

// NewFullNode creates a node that runs all services
func NewFullNode(config Config) *FullNode {
	// Create shared store
	chainStore := store.NewPersistentChainStore(config.DBName)

	return &FullNode{
		Store:  chainStore,
		Config: config,
		// Components will be initialized in Start()
	}
}

// Start initializes and starts all components (convenience method)
func (n *FullNode) Start() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		// 1. Initialize chain
		if err := n.InitializeChain(); err != nil {
			log.Printf("%s\tFailed to initialize chain: %v", n.Config.NodeID, err)
			ready <- false
			return
		}

		// 2. Start networking
		if !<-n.StartNetworking() {
			log.Printf("%s\tFailed to start networking", n.Config.NodeID)
			ready <- false
			return
		}

		// 3. Connect to seeds
		if !<-n.ConnectToSeeds() {
			log.Printf("%s\tFailed to connect to seeds", n.Config.NodeID)
			ready <- false
			return
		}

		// 4. Start discovery
		if !<-n.StartDiscovery() {
			log.Printf("%s\tFailed to start discovery", n.Config.NodeID)
			ready <- false
			return
		}

		// 5. Start sync
		if !<-n.StartSync() {
			log.Printf("%s\tFailed to start sync", n.Config.NodeID)
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

// InitializeChain checks if blockchain exists, if not initializes with genesis
func (n *FullNode) InitializeChain() error {
	chain, err := n.Store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get existing chain: %w", err)
	}

	if len(chain.Blocks) == 0 {
		// No existing chain, initialize with genesis
		if err := n.Store.AddBlock(blockchain.GenesisBlock); err != nil {
			return fmt.Errorf("failed to add genesis block: %w", err)
		}
		log.Printf("%s\tBlockchain initialized with genesis block", n.Config.NodeID)
	} else {
		// Chain already exists, log current state
		log.Printf("%s\tLoaded existing blockchain with %d blocks (height %d)",
			n.Config.NodeID, len(chain.Blocks), chain.Blocks[len(chain.Blocks)-1].Header.Height)
	}
	return nil
}

// StartNetworking starts just the network server (TCP listener)
func (n *FullNode) StartNetworking() <-chan bool {
	return n.startNetworking()
}

// StartDiscovery starts peer discovery (requires networking to be started)
func (n *FullNode) StartDiscovery() <-chan bool {
	ready := make(chan bool, 1)
	go func() {
		if n.NetworkServer == nil {
			log.Printf("%s\tNetwork server not started - call StartNetworking() first", n.Config.NodeID)
			ready <- false
			return
		}

		discoveryReady := n.NetworkServer.StartDiscovery()
		ready <- <-discoveryReady
	}()
	return ready
}

// ConnectToSeeds connects to configured seed peers (requires networking)
func (n *FullNode) ConnectToSeeds() <-chan bool {
	ready := make(chan bool, 1)
	go func() {
		if n.NetworkServer == nil {
			log.Printf("%s\tNetwork server not started - call StartNetworking() first", n.Config.NodeID)
			ready <- false
			return
		}

		if len(n.Config.SeedPeers) == 0 {
			log.Printf("%s\tNo seed peers configured", n.Config.NodeID)
			ready <- true
			return
		}

		// Connect to each seed peer
		var connectionTasks []<-chan bool
		for _, seedAddr := range n.Config.SeedPeers {
			connectionTasks = append(connectionTasks, n.NetworkServer.ConnectToPeer(seedAddr))
		}

		// Wait for all connection attempts to complete
		successCount := 0
		for _, task := range connectionTasks {
			if <-task {
				successCount++
			}
		}

		log.Printf("%s\tSeed connection attempts completed: %d/%d successful",
			n.Config.NodeID, successCount, len(n.Config.SeedPeers))
		ready <- successCount > 0 // Success if at least one connection worked
	}()
	return ready
}

// StartSync starts chain synchronization (requires networking and discovery)
func (n *FullNode) StartSync() <-chan bool {
	ready := make(chan bool, 1)
	go func() {
		if n.NetworkServer == nil {
			log.Printf("%s\tNetwork server not started - call StartNetworking() first", n.Config.NodeID)
			ready <- false
			return
		}

		// Start the periodic processes (cleanup and chain sync)
		err := n.NetworkServer.StartPeriodicProcesses()
		if err != nil {
			log.Printf("%s\tFailed to start periodic processes: %v", n.Config.NodeID, err)
			ready <- false
			return
		}

		log.Printf("%s\tChain synchronization active", n.Config.NodeID)
		ready <- true
	}()
	return ready
}

// startNetworking starts network server and signals when ready
func (n *FullNode) startNetworking() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		log.Printf("%s\tStarting network server on port %s", n.Config.NodeID, n.Config.Port)

		networkConfig := networking.Config{
			Port:              n.Config.Port,
			NodeID:            n.Config.NodeID,
			Store:             n.Store,
			SeedPeers:         n.Config.SeedPeers,
			ReqRespConfig:     networking.DefaultReqRespConfig(),
			PeerRequestConfig: networking.DefaultPeerRequestConfig(),
		}
		n.NetworkServer = networking.NewServer(networkConfig)

		err := n.NetworkServer.StartListener()
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

	// Close database connection
	if closer, ok := n.Store.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			log.Printf("%s\tError closing database: %v", n.Config.NodeID, err)
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
	go func() { <-networking.RelayTransaction(n.NetworkServer, tx) }()
	return nil
}
