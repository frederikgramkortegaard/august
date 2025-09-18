package node

import (
	"august/blockchain"
	"august/networking"
	"august/storage"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
)

// OrphanBlock represents a block waiting for its parent
type OrphanBlock struct {
	Block        *blockchain.Block
	Source       string
	ReceivedAt   time.Time
	ParentNeeded blockchain.Hash32
}

// NodeConfig holds all configuration for a full node
type NodeConfig struct {
	Port         string
	NodeID       string
	SeedPeers    []string
	DatabaseName string
	QueryPort    string // HTTP port for query API and miners (optional)
}

// FullNode orchestrates Peer Discovery and the rest of networking stuff
type FullNode struct {
	// Core blockchain storage
	Store *storage.Store

	// Configuration
	Config NodeConfig

	// Components (each package handles its own concern)
	NetworkServer *networking.Server // Network message handling
	Mempool       *Mempool           // Transaction pool

	// Orphan block management
	orphanBlocks   map[blockchain.Hash32]*OrphanBlock
	orphanBlocksMu sync.RWMutex

	// Goroutine management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewFullNode creates a node that runs all services
func NewFullNode(config NodeConfig) *FullNode {
	// Create shared store
	chainStore := storage.NewStore(config.DatabaseName)

	// Create mempool
	nodeMempool := NewMempool()

	// Create context for goroutine management
	ctx, cancel := context.WithCancel(context.Background())

	return &FullNode{
		Store:        chainStore,
		Config:       config,
		Mempool:      nodeMempool,
		orphanBlocks: make(map[blockchain.Hash32]*OrphanBlock),
		ctx:          ctx,
		cancel:       cancel,
		// Components will be initialized in Start()
	}
}

// Start initializes and starts all components
func (n *FullNode) Start() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		// 1. Start networking
		log.Printf("%s\tStarting network server on port %s", n.Config.NodeID, n.Config.Port)

		networkConfig := networking.NetworkConfig{
			Port:      n.Config.Port,
			NodeID:    n.Config.NodeID,
			Store:     n.Store,
			SeedPeers: n.Config.SeedPeers,
			Node:      n,
		}
		n.NetworkServer = networking.NewServer(networkConfig)

		err := n.NetworkServer.StartListener()
		if err != nil {
			log.Printf("%s\tFailed to start network server: %v", n.Config.NodeID, err)
			ready <- false
			return
		}

		// 2. Start discovery
		if !<-n.NetworkServer.StartDiscovery() {
			log.Printf("%s\tFailed to start discovery", n.Config.NodeID)
			ready <- false
			return
		}

		// 3. Start sync
		err = n.NetworkServer.StartPeriodicProcesses()
		if err != nil {
			log.Printf("%s\tFailed to start periodic processes: %v", n.Config.NodeID, err)
			ready <- false
			return
		}
		log.Printf("%s\tChain synchronization active", n.Config.NodeID)

		// 4. Start HTTP API for miners and queries if configured
		if n.Config.QueryPort != "" {
			go func() {
				port, _ := strconv.Atoi(n.Config.QueryPort)
				if err := StartQueryAPI(n, port); err != nil {
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
	if err := n.Store.Close(); err != nil {
		log.Printf("%s\tError closing database: %v", n.Config.NodeID, err)
	}

	log.Println("FullNode stopped successfully")
	return nil
}

// ProcessTransaction validates and processes a transaction (adding to mempool and relaying)
func (n *FullNode) ProcessTransaction(tx *blockchain.Transaction) error {
	if n.NetworkServer == nil {
		return fmt.Errorf("network server not initialized")
	}

	// Get current chain state for validation
	chain, err := n.Store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get chain state: %w", err)
	}

	// Validate transaction against current chain state
	if err := blockchain.ValidateTransaction(tx, chain.AccountStates); err != nil {
		return fmt.Errorf("transaction validation failed: %w", err)
	}

	// Add validated transaction to mempool
	if !n.Mempool.AddTransaction(*tx) {
		return fmt.Errorf("transaction rejected by mempool (duplicate or fee too low)")
	}

	txHash := tx.GetHash()
	log.Printf("%s\tTransaction added to mempool: %s", n.Config.NodeID, txHash.String())

	// Broadcast the transaction to all connected peers
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		select {
		case <-n.ctx.Done():
			return // Node is shutting down
		case <-networking.RelayTransaction(n.NetworkServer, tx):
			// Transaction relayed successfully
		}
	}()

	return nil
}

// ProcessHeader validates and processes a block header (deciding if we need the full block)
func (n *FullNode) ProcessHeader(header *blockchain.BlockHeader, sourcePeer string) error {
	if n.NetworkServer == nil {
		return fmt.Errorf("network server not initialized")
	}

	blockHash := header.GetHash()
	log.Printf("%s\tProcessing block header %s (height %d) from peer %s",
		n.Config.NodeID, blockHash.String(), header.Height, sourcePeer)

	// Get current chain to evaluate if we want this block
	ourChain, err := n.Store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get our chain: %w", err)
	}

	// Check if we already have this exact block
	if _, exists := ourChain.GetBlockByHash(blockHash); exists {
		log.Printf("%s\tIgnoring block header %s: already have this block", n.Config.NodeID, blockHash.String())
		return nil
	}

	// Evaluate if we should request this block
	var shouldRequest bool
	var reason string = "not needed"

	ourHeight := uint64(0)
	if len(ourChain.Blocks) > 0 {
		ourHeight = ourChain.Blocks[len(ourChain.Blocks)-1].Header.Height
	}

	if header.Height == ourHeight+1 {
		// This might be the next block in sequence
		if len(ourChain.Blocks) > 0 {
			ourTip := ourChain.Blocks[len(ourChain.Blocks)-1]
			if ourTip.Header.GetHash() == header.PreviousHash {
				shouldRequest, reason = true, "next block in sequence"
			}
		}
	} else if header.Height > ourHeight+1 {
		// We're significantly behind - initiate chain synchronization
		log.Printf("%s\tPeer is ahead by %d blocks (our: %d, peer: %d), initiating chain sync",
			n.Config.NodeID, header.Height-ourHeight, ourHeight, header.Height)

		// Request headers from height 1 to peer's height for headers-first sync
		go func() {
			headers, err := networking.RequestHeadersByHeight(n.NetworkServer, 1, header.Height)
			if err != nil {
				log.Printf("%s\tFailed to download headers for chain sync: %v", n.Config.NodeID, err)
				return
			}

			// Validate header chain
			if !blockchain.ValidateHeaderChain(headers) {
				log.Printf("%s\tInvalid header chain during sync, rejecting", n.Config.NodeID)
				return
			}

			// Convert headers to block hashes and request all blocks
			var blockHashes []string
			for _, h := range headers {
				hash := h.GetHash()
				hashStr := base64.StdEncoding.EncodeToString(hash[:])
				blockHashes = append(blockHashes, hashStr)
			}

			// Download all blocks
			blocks, err := n.NetworkServer.RequestBlocksByHash(blockHashes)
			if err != nil {
				log.Printf("%s\tFailed to download blocks for chain sync: %v", n.Config.NodeID, err)
				return
			}

			// Process each block sequentially
			for _, block := range blocks {
				done := n.ProcessBlock(block)
				<-done // Wait for each block to be processed
			}

			log.Printf("%s\tChain synchronization completed", n.Config.NodeID)
		}()
		return nil // Don't continue with single block processing
	} else if header.Height <= ourHeight {
		// Check if this could be a fork with more work
		if len(ourChain.Blocks) > 0 {
			ourTip := ourChain.Blocks[len(ourChain.Blocks)-1]
			if blockchain.CompareWork(header.TotalWork, ourTip.Header.TotalWork) > 0 {
				shouldRequest, reason = true, "potential fork with more work"
			}
		}
	}
	if shouldRequest {
		log.Printf("%s\tRequesting full block %s: %s", n.Config.NodeID, blockHash.String(), reason)

		// Request the full block through networking
		hashString := base64.StdEncoding.EncodeToString(blockHash[:])
		go func() {
			blocks, err := n.NetworkServer.RequestBlocksByHash([]string{hashString})
			if err != nil {
				log.Printf("%s\tFailed to request block %s from peers: %v", n.Config.NodeID, blockHash.String(), err)
				return
			}

			if len(blocks) == 0 {
				log.Printf("%s\tNo block returned for %s from peers", n.Config.NodeID, blockHash.String())
				return
			}

			log.Printf("%s\tReceived the block we requested: %s", n.Config.NodeID, blockHash.String())
			// Process the requested block
			<-n.ProcessBlock(blocks[0])
		}()
	} else {
		log.Printf("%s\tIgnoring block header %s: %s", n.Config.NodeID, blockHash.String(), reason)
	}

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
		blockHash := block.Header.GetHash()

		// Determine exclude address for relay
		var excludeAddr string
		if len(excludePeerAddr) > 0 {
			excludeAddr = excludePeerAddr[0]
		}

		// Get current chain and make a deep copy for validation
		originalChain, err := n.Store.GetChain()
		if err != nil {
			log.Printf("%s\tFailed to get chain for block %s: %v", n.Config.NodeID, blockHash.String(), err)
			return
		}

		// Make a deep copy to validate on without affecting the original
		chainCopy := originalChain.DeepCopy()
		if chainCopy == nil {
			log.Printf("%s\tFailed to create chain copy for block %s", n.Config.NodeID, blockHash.String())
			return
		}

		// Check if we have the parent block before attempting validation
		// Special case: if PreviousHash equals genesis hash and we have genesis, parent exists
		genesisHash := blockchain.GenesisBlock.Header.GetHash()
		isGenesisChild := block.Header.PreviousHash == genesisHash && len(chainCopy.Blocks) > 0

		// Also check for all-zeros hash (should only be genesis itself)
		isAllZeros := block.Header.PreviousHash == blockchain.Hash32{}
		isGenesisBlock := blockHash == genesisHash

		_, parentExists := chainCopy.GetBlockByHash(block.Header.PreviousHash)

		// DEBUG: Log the chain state and checks
		log.Printf("%s\tDEBUG: Processing block %s", n.Config.NodeID, blockHash.String())
		log.Printf("%s\tDEBUG: Chain has %d blocks", n.Config.NodeID, len(chainCopy.Blocks))
		log.Printf("%s\tDEBUG: isGenesisBlock=%t, isGenesisChild=%t, isAllZeros=%t, parentExists=%t",
			n.Config.NodeID, isGenesisBlock, isGenesisChild, isAllZeros, parentExists)
		if len(chainCopy.Blocks) > 0 {
			firstBlockHash := chainCopy.Blocks[0].Header.GetHash()
			log.Printf("%s\tDEBUG: First block in chain: %s", n.Config.NodeID, firstBlockHash.String())
		}

		if !parentExists && !isGenesisChild && !isGenesisBlock {
			// This is an orphan block - we don't have its parent
			log.Printf("%s\tBlock %s is orphan, missing parent %s. Adding to candidate blocks.",
				n.Config.NodeID, blockHash.String(), block.Header.PreviousHash.String())

			// Store in orphan blocks
			n.AddOrphanBlock(block, excludeAddr, block.Header.PreviousHash)

			// Never request all-zeros hash from peers (genesis parent doesn't exist)
			if isAllZeros {
				log.Printf("%s\tCannot request all-zeros parent hash from peers", n.Config.NodeID)
				return
			}

			// Request missing parent block from peers
			log.Printf("%s\tNeed to request parent block %s from peers", n.Config.NodeID, block.Header.PreviousHash.String())
			hashString := base64.StdEncoding.EncodeToString(block.Header.PreviousHash[:])
			go func() {
				blocks, err := n.NetworkServer.RequestBlocksByHash([]string{hashString})
				if err != nil {
					log.Printf("%s\tFailed to request parent block %s from peers: %v", n.Config.NodeID, block.Header.PreviousHash.String(), err)
					return
				}

				if len(blocks) == 0 {
					log.Printf("%s\tNo parent block returned for %s from peers", n.Config.NodeID, block.Header.PreviousHash.String())
					return
				}

				// Process the parent block - this should connect orphan blocks
				<-n.ProcessBlock(blocks[0])
			}()
			return
		}

		// We have the parent block, proceed with validation
		err = blockchain.ValidateAndApplyBlock(block, chainCopy)

		// Check if this is a chain switch request (fork detected)
		if details, ok := err.(blockchain.ErrSwitchChain); ok {
			log.Printf("%s\tBlock %s detected fork, need to check for chain reorganization", n.Config.NodeID, blockHash.String())

			currentTipWork := chainCopy.Blocks[len(chainCopy.Blocks)-1].Header.TotalWork
			newBlockWork := details.Block.Header.TotalWork
			workComparison := blockchain.CompareWork(newBlockWork, currentTipWork)

			log.Printf("%s\tWork comparison: new block work=%s, current tip work=%s, comparison=%d",
				n.Config.NodeID, newBlockWork, currentTipWork, workComparison)

			// Check if current chain has more work
			if workComparison <= 0 {
				log.Printf("%s\tCurrent chain has more total work, ignoring block %s", n.Config.NodeID, blockHash.String())
				return
			}

			// Perform Chain Switch - fork has more work
			// Create new chain up to the common ancestor (fork point)
			newChain := &blockchain.Chain{
				Blocks:        chainCopy.Blocks[:details.CommonAncestor.Header.Height+1],
				AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
			}

			// Rebuild account states up to the fork point
			for _, block := range newChain.Blocks {
				for _, tx := range block.Transactions {
					blockchain.ValidateAndApplyTransaction(&tx, newChain.AccountStates)
				}
			}

			// Now apply the fork block using ValidateAndApplyBlock
			err = blockchain.ValidateAndApplyBlock(details.Block, newChain)
			if err != nil {
				log.Printf("%s\tFailed to apply fork block %s: %v", n.Config.NodeID, blockHash.String(), err)
				return
			}

			chainCopy = newChain
			log.Printf("%s\tReorganized Chain", n.Config.NodeID)

			// Clean mempool after chain reorganization
			n.wg.Add(1)
			go func() {
				defer n.wg.Done()
				select {
				case <-n.ctx.Done():
					return
				default:
					// Revalidate all transactions against new chain state
					invalidated := n.Mempool.RevalidateTransactions(chainCopy.AccountStates)
					if invalidated > 0 {
						log.Printf("%s\tRemoved %d invalid transactions from mempool after chain reorganization", n.Config.NodeID, invalidated)
					}
				}
			}()

			// Other validation errors
		} else if err != nil {
			log.Printf("%s\tBlock %s validation failed: %v", n.Config.NodeID, blockHash.String(), err)
			return
		}

		// Block validation succeeded and was added to the copy, now atomically replace the chain
		if err := n.Store.ReplaceChain(chainCopy); err != nil {
			log.Printf("%s\tFailed to replace chain with validated block %s: %v", n.Config.NodeID, blockHash.String(), err)
			return
		}

		// Block successfully added to main chain
		log.Printf("%s\tBlock %s added to main chain", n.Config.NodeID, blockHash.String())

		// Clean mempool after new block is added
		n.wg.Add(1)
		go func() {
			defer n.wg.Done()
			select {
			case <-n.ctx.Done():
				return
			default:
				// Remove transactions that were included in this block from mempool
				removed := n.Mempool.RemoveTransactions(block.Transactions)
				if removed > 0 {
					log.Printf("%s\tRemoved %d transactions from mempool (included in block %s)", n.Config.NodeID, removed, blockHash.String())
				}

				// Revalidate remaining transactions against new chain state
				invalidated := n.Mempool.RevalidateTransactions(chainCopy.AccountStates)
				if invalidated > 0 {
					log.Printf("%s\tRemoved %d invalid transactions from mempool after block %s", n.Config.NodeID, invalidated, blockHash.String())
				}
			}
		}()

		// Relay the block header to connected peers (headers-first approach)
		n.wg.Add(1)
		go func() {
			defer n.wg.Done()
			select {
			case <-n.ctx.Done():
				return
			case <-networking.RelayBlockHeader(n.NetworkServer, &block.Header, excludeAddr):
				// Header relayed successfully
			}
		}()

		// Try to connect any orphan blocks that might now be connectible
		if err := n.TryConnectOrphanBlocks(); err != nil {
			log.Printf("%s\tError connecting orphan blocks: %v", n.Config.NodeID, err)
		}
	}()

	return complete
}

// GetChainHead returns information about the current chain head
func (n *FullNode) GetChainHead() blockchain.ChainHead {
	return n.Store.GetChainHead()
}

// GetChain returns the current blockchain state (required by NodeAPI interface)
func (n *FullNode) GetChain() (*blockchain.Chain, error) {
	return n.Store.GetChain()
}

// GetMempool returns the mempool instance (required by NodeAPI interface)
func (n *FullNode) GetMempool() *Mempool {
	return n.Mempool
}

// AddOrphanBlock stores an orphan block waiting for its parent
func (n *FullNode) AddOrphanBlock(block *blockchain.Block, source string, parentHash blockchain.Hash32) {
	n.orphanBlocksMu.Lock()
	defer n.orphanBlocksMu.Unlock()

	blockHash := block.Header.GetHash()
	n.orphanBlocks[blockHash] = &OrphanBlock{
		Block:        block,
		Source:       source,
		ReceivedAt:   time.Now(),
		ParentNeeded: parentHash,
	}
}

// TryConnectOrphanBlocks attempts to connect all orphan blocks by trying to validate and apply them
func (n *FullNode) TryConnectOrphanBlocks() error {
	n.orphanBlocksMu.Lock()
	// Make a copy to avoid concurrent modification
	orphansCopy := make(map[blockchain.Hash32]*OrphanBlock)
	for hash, orphan := range n.orphanBlocks {
		orphansCopy[hash] = orphan
	}
	n.orphanBlocksMu.Unlock()

	// Try to process all orphan blocks from the copy
	for hash, orphan := range orphansCopy {
		go func(blockHash blockchain.Hash32, orphanBlock *OrphanBlock) {
			done := n.ProcessBlock(orphanBlock.Block, orphanBlock.Source)
			<-done
			// If processing succeeds, ProcessBlock will handle removing it from the orphans map
		}(hash, orphan)
	}

	return nil
}
