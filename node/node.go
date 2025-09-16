package node

import (
	"august/blockchain"
	"august/mempool"
	"august/networking"
	"august/storage"
	. "august/types"
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
)

// Type aliases for types from other packages

// NewFullNode creates a node that runs all services
func NewFullNode(config Config) *FullNode {

	// Create shared store
	chainStore := storage.NewPersistentChainStore(config.DatabaseName)

	// Create mempool with configuration
	mempoolConfig := mempool.Config{
		MaxSize:   config.MaxMempoolSize,
		MaxExpiry: config.MempoolExpiry,
	}
	nodeMempool := mempool.NewMempool(mempoolConfig)

	// Create context for goroutine management
	ctx, cancel := context.WithCancel(context.Background())

	return &FullNode{
		Store:   chainStore,
		Config:  config,
		Mempool: nodeMempool,
		ctx:     ctx,
		cancel:  cancel,
		// Components will be initialized in Start()
	}
}

// Start initializes and starts all components (convenience method)
func (n *FullNode) Start() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		// 1. Initialize chain
		if err := n.InitializeChain(); err != nil {
			Log("CORE", n.Config.NodeID, "Failed to initialize chain: %v", err)
			ready <- false
			return
		}

		// 2. Start networking
		if !<-n.StartNetworking() {
			Log("NET", n.Config.NodeID, "Failed to start networking")
			ready <- false
			return
		}

		// 3. Connect to seeds
		if !<-n.ConnectToSeeds() {
			Log("NET", n.Config.NodeID, "Failed to connect to seeds")
			ready <- false
			return
		}

		// 4. Start discovery
		if !<-n.StartDiscovery() {
			Log("DISC", n.Config.NodeID, "Failed to start discovery")
			ready <- false
			return
		}

		// 5. Start sync
		if !<-n.StartSync() {
			Log("SYNC", n.Config.NodeID, "Failed to start sync")
			ready <- false
			return
		}

		// 6. Start HTTP API for miners and queries if configured
		if n.Config.QueryPort != "" {
			go func() {
				port, _ := strconv.Atoi(n.Config.QueryPort)
				if err := n.StartQueryAPI(port); err != nil {
					log.Printf("%s\tQuery API server failed: %v", n.Config.NodeID, err)
				}
			}()
		}

		Log("CORE", n.Config.NodeID, "Full node started: Network on :%s", n.Config.Port)

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
		Log("CORE", n.Config.NodeID, "Blockchain initialized with genesis block")
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

		Log("SYNC", n.Config.NodeID, "Chain synchronization active")
		ready <- true
	}()
	return ready
}

// startNetworking starts network server and signals when ready
func (n *FullNode) startNetworking() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		Log("NET", n.Config.NodeID, "Starting network server on port %s", n.Config.Port)

		networkConfig := networking.Config{
			Port:                 n.Config.Port,
			NodeID:               n.Config.NodeID,
			Store:                n.Store,
			SeedPeers:            n.Config.SeedPeers,
			ReqRespConfig:        networking.DefaultReqRespConfig(),
			PeerRequestConfig:    networking.DefaultPeerRequestConfig(),
			TransactionProcessor: n.SubmitTransaction, // Process incoming transactions through mempool
		}
		n.NetworkServer = networking.NewNetwork(networkConfig, n)

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

	// Signal all goroutines to stop
	if n.cancel != nil {
		n.cancel()
	}

	// Stop network server
	if n.NetworkServer != nil {
		if err := n.NetworkServer.Stop(); err != nil {
			log.Printf("%s\tError stopping network server: %v", n.Config.NodeID, err)
		}
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("%s\tAll goroutines stopped gracefully", n.Config.NodeID)
	case <-time.After(5 * time.Second):
		log.Printf("%s\tTimeout waiting for goroutines to stop", n.Config.NodeID)
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
func (n *FullNode) GetNetworkServer() *networking.Network {
	return n.NetworkServer
}

// GetNodeID returns the node's ID
func (n *FullNode) GetNodeID() string {
	return n.Config.NodeID
}

// GetChainHead returns information about the current chain head
func (n *FullNode) GetChainHead() blockchain.ChainHead {
	chain, err := n.Store.GetChain()
	if err != nil || len(chain.Blocks) == 0 {
		// Return genesis/empty chain info
		return blockchain.ChainHead{
			Height:    0,
			Hash:      blockchain.Hash32{},
			TotalWork: "0",
		}
	}

	lastBlock := chain.Blocks[len(chain.Blocks)-1]
	hash := blockchain.HashBlockHeader(&lastBlock.Header)

	return blockchain.ChainHead{
		Height:    lastBlock.Header.Height,
		Hash:      hash,
		TotalWork: lastBlock.Header.TotalWork,
	}
}

// GetChain returns the current blockchain state (required by queryapi.Node interface)
func (n *FullNode) GetChain() (*blockchain.Chain, error) {
	return n.Store.GetChain()
}

// GetMempool returns the mempool instance (required by queryapi.Node interface)
func (n *FullNode) GetMempool() interface {
	GetTransactions(limit int) []blockchain.Transaction
} {
	return n.Mempool
}
