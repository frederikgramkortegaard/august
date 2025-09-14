package node

import (
	"august/blockchain"
	"august/mempool"
	"august/networking"
	store "august/storage"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"time"
)

// Config holds all configuration for a full node
type Config struct {
	Port           string
	NodeID         string
	SeedPeers      []string
	DBName         string
	QueryPort      string        // HTTP port for query API and miners (optional)
	MaxMempoolSize int           // Maximum transactions in mempool (default: 1000)
	MempoolExpiry  time.Duration // Maximum age for mempool transactions (default: 7 days)
}

// FullNode orchestrates Peer Discovery and the rest of networking stuff
type FullNode struct {
	// Core blockchain storage
	Store store.ChainStore

	// Configuration
	Config Config

	// Components (each package handles its own concern)
	NetworkServer *networking.Server // Network message handling
	Mempool       *mempool.Mempool   // Transaction pool
}

// NewFullNode creates a node that runs all services
func NewFullNode(config Config) *FullNode {
	// Create shared store
	chainStore := store.NewPersistentChainStore(config.DBName)

	// Create mempool with configuration
	mempoolConfig := mempool.Config{
		MaxSize:   config.MaxMempoolSize,
		MaxExpiry: config.MempoolExpiry,
	}
	nodeMempool := mempool.NewMempool(mempoolConfig)

	return &FullNode{
		Store:   chainStore,
		Config:  config,
		Mempool: nodeMempool,
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

		// 6. Start HTTP API for miners and queries if configured
		if n.Config.QueryPort != "" {
			go func() {
				port, _ := strconv.Atoi(n.Config.QueryPort)
				if err := n.StartQueryAPI(port); err != nil {
					log.Printf("%s\tQuery API server failed: %v", n.Config.NodeID, err)
				}
			}()
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

		// Set the block processor callback to use our node's ProcessBlock method
		n.NetworkServer.SetBlockProcessor(n.ProcessBlock)

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

	// Get current chain state for validation
	chain, err := n.Store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get chain state: %w", err)
	}

	// Try to add transaction to mempool (includes validation)
	if !n.Mempool.AddTransaction(*tx, chain.AccountStates) {
		return fmt.Errorf("transaction rejected by mempool (invalid, duplicate, or fee too low)")
	}

	txHash := blockchain.HashTransaction(tx)
	log.Printf("%s\tTransaction added to mempool: %x", n.Config.NodeID, txHash[:8])

	// Broadcast the transaction to all connected peers
	go func() {
		// @TODO: Implement RelayTransaction in networking package
		// <-networking.RelayTransaction(n.NetworkServer, tx)
	}()

	return nil
}

// SubmitBlock accepts an already-mined block from external miners
func (n *FullNode) SubmitBlock(block *blockchain.Block) error {
	if n.NetworkServer == nil {
		return fmt.Errorf("network server not initialized")
	}

	// Process through the node's block processing pipeline
	go func() {
		<-n.ProcessBlock(block)
	}()

	return nil
}

// ProcessBlock attempts to add a block to the main chain, handling orphans
// Returns a completion channel that will be closed when processing completes
// excludePeerAddr: if provided, this peer will be excluded from relay (used when block came from a peer)
func (n *FullNode) ProcessBlock(block *blockchain.Block, excludePeerAddr ...string) <-chan struct{} {
	complete := make(chan struct{})

	go func() {
		defer close(complete)

		if block == nil {
			return
		}

		blockHash := blockchain.HashBlockHeader(&block.Header)

		// Determine exclude address for relay
		var excludeAddr string
		if len(excludePeerAddr) > 0 {
			excludeAddr = excludePeerAddr[0]
		}

		// Get current chain and make a deep copy for validation
		originalChain, err := n.Store.GetChain()
		if err != nil {
			log.Printf("%s\tFailed to get chain for block %x: %v", n.Config.NodeID, blockHash[:8], err)
			return
		}

		// Make a deep copy to validate on without affecting the original
		chainCopy := originalChain.DeepCopy()
		if chainCopy == nil {
			log.Printf("%s\tFailed to create chain copy for block %x", n.Config.NodeID, blockHash[:8])
			return
		}

		// Try to validate and apply block to the copy (this also adds the block to the chain)
		if err := blockchain.ValidateAndApplyBlock(block, chainCopy); err != nil {
			// Check if this is a missing parent error (orphan block)
			if missingParentErr, ok := err.(blockchain.ErrMissingParent); ok {
				log.Printf("%s\tBlock %x is orphan, missing parent %x. Adding to candidate blocks.",
					n.Config.NodeID, blockHash[:8], missingParentErr.Hash[:8])

				// Store in candidate blocks using consensus manager
				n.NetworkServer.GetConsensusManager().AddCandidateBlock(block, excludeAddr, missingParentErr.Hash)

				// Request missing parent block from multiple peers
				log.Printf("%s\tNeed to request parent block %x from peers", n.Config.NodeID, missingParentErr.Hash[:8])

				connectedPeers := n.NetworkServer.GetPeerManager().GetConnectedPeers()
				if len(connectedPeers) > 0 {
					hashString := base64.StdEncoding.EncodeToString(missingParentErr.Hash[:])
					go func() {
						blocks, err := networking.RequestBlocksByHash(n.NetworkServer, []string{hashString})
						if err != nil {
							log.Printf("%s\tFailed to request parent block %x from peers: %v", n.Config.NodeID, missingParentErr.Hash[:8], err)
							return
						}

						if len(blocks) == 0 {
							log.Printf("%s\tNo parent block returned for %x from peers", n.Config.NodeID, missingParentErr.Hash[:8])
							return
						}

						// Process the parent block - this should connect orphan blocks
						<-n.ProcessBlock(blocks[0])
					}()
				}
				return

				// Check if this is a chain switch request (fork detected)
			} else if details, ok := err.(blockchain.ErrSwitchChain); ok {
				log.Printf("%s\tBlock %x detected fork, need to check for chain reorganization", n.Config.NodeID, blockHash[:8])

				// Check if current chain has more work
				if blockchain.CompareWork(details.Block.Header.TotalWork, chainCopy.Blocks[len(chainCopy.Blocks)-1].Header.TotalWork) <= 0 {
					log.Printf("%s\tCurrent chain has more total work, ignoring block %x", n.Config.NodeID, blockHash[:8])
					return
				}

				// Perform Chain Switch - fork has more work
				// Build new chain up to the fork point
				newBlocks := chainCopy.Blocks[:details.Block.Header.Height]
				newBlocks = append(newBlocks, details.Block)

				// Create new chain and rebuild account states from genesis
				newChain := &blockchain.Chain{
					Blocks:        newBlocks,
					AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
				}

				// Rebuild account states by processing all transactions
				for _, block := range newBlocks {
					for _, tx := range block.Transactions {
						blockchain.ValidateAndApplyTransaction(&tx, newChain.AccountStates)
					}
				}

				chainCopy = newChain
				log.Printf("%s\tReorganized Chain", n.Config.NodeID)

				// Clean mempool after chain reorganization
				go func() {
					// Revalidate all transactions against new chain state
					invalidated := n.Mempool.RevalidateTransactions(chainCopy.AccountStates)
					if invalidated > 0 {
						log.Printf("%s\tRemoved %d invalid transactions from mempool after chain reorganization", n.Config.NodeID, invalidated)
					}
				}()

			} else {
				// Other validation errors
				log.Printf("%s\tBlock %x validation failed: %v", n.Config.NodeID, blockHash[:8], err)
				return
			}
		}

		// Block validation succeeded and was added to the copy, now atomically replace the chain
		if err := n.Store.ReplaceChain(chainCopy); err != nil {
			log.Printf("%s\tFailed to replace chain with validated block %x: %v", n.Config.NodeID, blockHash[:8], err)
			return
		}

		// Block successfully added to main chain
		log.Printf("%s\tBlock %x added to main chain", n.Config.NodeID, blockHash[:8])

		// Clean mempool after new block is added
		go func() {
			// Remove transactions that were included in this block from mempool
			removed := n.Mempool.RemoveTransactions(block.Transactions)
			if removed > 0 {
				log.Printf("%s\tRemoved %d transactions from mempool (included in block %x)", n.Config.NodeID, removed, blockHash[:8])
			}

			// Revalidate remaining transactions against new chain state
			invalidated := n.Mempool.RevalidateTransactions(chainCopy.AccountStates)
			if invalidated > 0 {
				log.Printf("%s\tRemoved %d invalid transactions from mempool after block %x", n.Config.NodeID, invalidated, blockHash[:8])
			}
		}()

		// Relay the block header to connected peers (headers-first approach)
		go func() { <-networking.RelayBlockHeader(n.NetworkServer, &block.Header, excludeAddr) }()

		// Try to connect any candidate blocks that might now be connectible
		if err := n.NetworkServer.GetConsensusManager().TryConnectCandidateBlocks(block); err != nil {
			log.Printf("%s\tError connecting candidate blocks: %v", n.Config.NodeID, err)
		}
	}()

	return complete
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

