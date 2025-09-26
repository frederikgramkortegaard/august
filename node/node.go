package node

import (
	"august/blockchain"
	"august/config"
	"august/networking"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sort"
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
	Port      string
	NodeID    string
	SeedPeers []string
	QueryPort string // HTTP port for query API and miners (optional)

}

// Node orchestrates Peer Discovery and the rest of networking stuff
type Node struct {

	// Configuration
	Config NodeConfig

	// Components (each package handles its own concern)
	NetworkServer *networking.Server // Network message handling
	Mempool       *Mempool           // Transaction pool

	// Chain Management
	chains          map[blockchain.Hash32]*blockchain.Chain // keyed by tip hash
	chainsMu        sync.RWMutex
	currentChainTip blockchain.Hash32 // points to which chain is current

	// Global header index for reliable header walking across chains
	headersMap   map[blockchain.Hash32]*blockchain.BlockHeader
	headersMapMu sync.RWMutex

	// Orphan block management
	orphanBlocks   map[blockchain.Hash32]*OrphanBlock
	orphanBlocksMu sync.RWMutex

	orphanHeaders   map[blockchain.Hash32][]*blockchain.BlockHeader // keyed by PreviousHash
	orphanHeadersMu sync.RWMutex

	// Header processing synchronization to prevent race conditions
	headerProcessingMu sync.Mutex

	// API submitted blocks (temporary storage until processed)
	submittedBlocks   map[blockchain.Hash32]*blockchain.Block
	submittedBlocksMu sync.RWMutex

	// Goroutine management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (n *Node) GetCurrentChain() *blockchain.Block {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	if n.currentChainTip == (blockchain.Hash32{}) {
		return nil
	}

	chain := n.chains[n.currentChainTip]
	if chain == nil {
		return nil
	}

	return chain.GetTip()
}

func (n *Node) GetBlock(hash blockchain.Hash32) *blockchain.Block {
	// First check submitted blocks (from API)
	n.submittedBlocksMu.RLock()
	if block := n.submittedBlocks[hash]; block != nil {
		n.submittedBlocksMu.RUnlock()
		return block
	}
	n.submittedBlocksMu.RUnlock()

	// Then search through all chains for the block
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	for _, chain := range n.chains {
		if block := chain.GetBlock(hash); block != nil {
			return block
		}
	}
	return nil
}

func (n *Node) GetHeader(hash blockchain.Hash32) *blockchain.BlockHeader {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	// Search through all chains for the header
	for _, chain := range n.chains {
		if header := chain.GetHeader(hash); header != nil {
			return header
		}
	}
	return nil
}

func (n *Node) GetBlockAtHeight(height uint64) *blockchain.Block {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	if n.currentChainTip == (blockchain.Hash32{}) {
		return nil
	}

	chain := n.chains[n.currentChainTip]
	if chain == nil {
		return nil
	}

	return chain.GetBlockAtHeight(height)
}

// NewNode creates a node that runs all services
func NewNode(config NodeConfig) *Node {
	// Create shared store

	// Create mempool
	nodeMempool := NewMempool()

	// Create context for goroutine management
	ctx, cancel := context.WithCancel(context.Background())

	node := &Node{
		Config:          config,
		Mempool:         nodeMempool,
		chains:          make(map[blockchain.Hash32]*blockchain.Chain),
		headersMap:      make(map[blockchain.Hash32]*blockchain.BlockHeader),
		orphanBlocks:    make(map[blockchain.Hash32]*OrphanBlock),
		orphanHeaders:   make(map[blockchain.Hash32][]*blockchain.BlockHeader),
		submittedBlocks: make(map[blockchain.Hash32]*blockchain.Block),
		ctx:             ctx,
		cancel:          cancel,
		// Components will be initialized in Start()
	}

	// Initialize with genesis chain
	genesisChain := blockchain.NewChain()
	genesisChain.AddHeader(&blockchain.GenesisBlock.Header)
	genesisChain.AddBlock(blockchain.GenesisBlock) // Add the full genesis block
	genesisHash := blockchain.GenesisBlock.Header.GetHash()
	node.chains[genesisHash] = genesisChain
	node.currentChainTip = genesisHash

	// Add genesis header to global headers map
	node.headersMap[genesisHash] = &blockchain.GenesisBlock.Header

	return node
}

// Start initializes and starts all components
func (n *Node) Start() <-chan bool {
	ready := make(chan bool, 1)

	go func() {
		// 1. Start networking
		log.Printf("%s\tStarting network server on port %s", n.Config.NodeID, n.Config.Port)

		networkConfig := networking.NetworkConfig{
			Port:      n.Config.Port,
			NodeID:    n.Config.NodeID,
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

// Stop gracefully shuts down the Node
func (n *Node) Stop() error {
	log.Printf("%s\tStopping Node...", n.Config.NodeID)

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

	log.Println("Node stopped successfully")
	return nil
}

// ProcessTransaction validates and processes a transaction (adding to mempool and relaying)
func (n *Node) ProcessTransaction(tx *blockchain.Transaction) error {
	if n.NetworkServer == nil {
		return fmt.Errorf("network server not initialized")
	}

	// Get current account states for validation
	n.chainsMu.RLock()
	var statesCopy map[blockchain.PublicKey]*blockchain.AccountState
	if n.currentChainTip != (blockchain.Hash32{}) {
		if currentChain := n.chains[n.currentChainTip]; currentChain != nil {
			statesCopy = currentChain.GetAccountStates()
		}
	}
	if statesCopy == nil {
		statesCopy = make(map[blockchain.PublicKey]*blockchain.AccountState)
	}
	n.chainsMu.RUnlock()

	// Validate transaction against current account states
	if err := blockchain.ValidateTransaction(tx, statesCopy); err != nil {
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

// ProcessHeader validates and processes a block header
func (n *Node) ProcessHeader(header *blockchain.BlockHeader, sourcePeer string) error {
	// Serialize header processing to prevent race conditions with current chain updates
	n.headerProcessingMu.Lock()
	defer n.headerProcessingMu.Unlock()

	blockHash := header.GetHash()
	log.Printf("%s\tProcessing header %s (height %d) from peer %s", n.Config.NodeID, blockHash.String()[:8], header.Height, sourcePeer)

	// Validate the header first (no locks needed)
	if err := blockchain.ValidateHeaderStructure(header); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	// Check if we already have this header and find parent chain (with read lock)
	n.chainsMu.RLock()
	var parentChain *blockchain.Chain
	var parentChainTip blockchain.Hash32
	var headerExists bool

	// Check if we already have this header in any chain
	for _, chain := range n.chains {
		if chain.GetHeader(blockHash) != nil {
			headerExists = true
			break
		}
	}

	if !headerExists {
		// Find which chain this header extends (if any)
		// Only extend a chain if the parent is the TIP of that chain
		for tipHash, chain := range n.chains {
			if tipHash == header.PreviousHash {
				if parentHeader := chain.GetHeader(header.PreviousHash); parentHeader != nil {
					// Found parent as tip of this chain, validate against parent
					if err := blockchain.ValidateHeaderTotalWork(header, parentHeader.TotalWork); err != nil {
						n.chainsMu.RUnlock()
						return fmt.Errorf("invalid header against parent: %w", err)
					}
					parentChain = chain
					parentChainTip = tipHash
					break
				}
			}
		}
	}
	n.chainsMu.RUnlock()

	// Early return if header already exists
	if headerExists {
		return nil
	}

	// Early return for orphan headers
	if parentChain == nil {
		// Check if we have the parent block in any chain (potential new fork)
		n.chainsMu.RLock()
		var parentBlock *blockchain.Block
		var sourceChain *blockchain.Chain
		for _, chain := range n.chains {
			if block := chain.GetBlock(header.PreviousHash); block != nil {
				parentBlock = block
				sourceChain = chain
				break
			}
		}
		n.chainsMu.RUnlock()

		if parentBlock != nil {
			// We have the parent block - create new candidate chain up to common ancestor
			log.Printf("%s\tHeader %s creates new candidate chain (fork from block %s)",
				n.Config.NodeID, blockHash.String(), header.PreviousHash.String())

			// Create new candidate chain by validating and applying blocks from genesis to parent
			newChain := blockchain.NewChain()

			// Check if we have all blocks needed from genesis to parent
			var missingBlocks []blockchain.Hash32
			currentHash := parentBlock.Header.GetHash()

			// Walk back to genesis checking for missing blocks
			for {
				if currentHash == blockchain.GenesisBlock.Header.GetHash() {
					break // Reached genesis, we always have this
				}

				block := sourceChain.GetBlock(currentHash)
				if block == nil {
					missingBlocks = append(missingBlocks, currentHash)
				}

				// Get parent hash for next iteration
				if block != nil {
					currentHash = block.Header.PreviousHash
				} else {
					// If we don't have the block, we can't walk further back
					// We'll need to fetch it and try again later
					break
				}
			}

			if len(missingBlocks) > 0 {
				// We're missing blocks - queue this header for later processing after fetch
				log.Printf("%s\tMissing %d blocks for new chain creation, fetching blocks", n.Config.NodeID, len(missingBlocks))

				// Convert to base64 strings for request
				var hashStrings []string
				for _, hash := range missingBlocks {
					hashStrings = append(hashStrings, base64.StdEncoding.EncodeToString(hash[:]))
				}

				// Fetch blocks asynchronously and retry header processing
				go func() {
					blocksChan := n.NetworkServer.RequestBlocksByHash(hashStrings)
					blocks := <-blocksChan

					if blocks == nil || len(blocks) != len(hashStrings) {
						log.Printf("%s\tFailed to fetch blocks for new chain creation", n.Config.NodeID)
						return
					}

					// Lock to add fetched blocks to source chain
					n.chainsMu.Lock()
					for _, block := range blocks {
						sourceChain.AddBlock(block)
					}
					n.chainsMu.Unlock()

					// Now retry processing this header (it should succeed now since we have all blocks)
					n.ProcessHeader(header, sourcePeer)
				}()

				return nil // Don't process now, wait for async completion
			}

			// We have all blocks - build the chain now
			var blocksToApply []*blockchain.Block
			currentBlock := parentBlock

			// Walk back to genesis collecting blocks
			for currentBlock != nil {
				blocksToApply = append([]*blockchain.Block{currentBlock}, blocksToApply...)
				if currentBlock.Header.Height == 0 {
					break // Reached genesis
				}
				currentBlock = sourceChain.GetBlock(currentBlock.Header.PreviousHash)
			}

			// Apply blocks in order from genesis to parent
			for _, block := range blocksToApply {
				newAccountStates, err := blockchain.ValidateBlock(block, newChain)
				if err != nil {
					log.Printf("%s\tFailed to validate block in new chain creation: %v", n.Config.NodeID, err)
					return fmt.Errorf("failed to build candidate chain: %w", err)
				}

				if err := blockchain.ApplyBlock(block, newChain, newAccountStates); err != nil {
					log.Printf("%s\tFailed to apply block in new chain creation: %v", n.Config.NodeID, err)
					return fmt.Errorf("failed to build candidate chain: %w", err)
				}

				// Try to connect orphan headers that might now have their parent
				n.tryConnectOrphanChildren(block.Header.GetHash())
			}

			// Add to chains map with parent as tip
			parentHash := parentBlock.Header.GetHash()
			n.chains[parentHash] = newChain

			log.Printf("%s\tCreated new candidate chain with tip %s, continuing with header processing",
				n.Config.NodeID, parentHash.String())

			// Set parentChain to the new chain and continue processing normally
			parentChain = newChain
			parentChainTip = parentHash
		} else {
			// No parent found anywhere - true oprhan
			// Log outside of locks
			log.Printf("%s\tHeader %s is orphan (parent not found), adding to orphan headers", n.Config.NodeID, blockHash.String())

			// Add to orphan headers
			n.orphanHeadersMu.Lock()
			n.orphanHeaders[header.PreviousHash] = append(n.orphanHeaders[header.PreviousHash], header)
			n.orphanHeadersMu.Unlock()

			// Only request parent headers if this header didn't come from orphan processing
			// This prevents infinite loops where orphan processing triggers more orphan processing
			if sourcePeer != "orphan-connection" {
				// Request parent header to find common ancestor, but only if we don't already have it
				parentHeader := n.GetHeader(header.PreviousHash)
				if parentHeader == nil {
					log.Printf("%s\tOrphan header at height %d, requesting parent header and block to find common ancestor",
						n.Config.NodeID, header.Height)

					// Request parent header and block recursively
					go func() {
						parentHashStr := base64.StdEncoding.EncodeToString(header.PreviousHash[:])

						// First get the header
						headers, err := networking.RequestHeadersByHash(n.NetworkServer, []string{parentHashStr})
						if err == nil && len(headers) > 0 {
							// Process the parent header first - this will recursively handle any missing ancestors
							n.ProcessHeader(&headers[0], sourcePeer)

							// Then get the corresponding block and add it to candidate chains
							blocksChan := n.NetworkServer.RequestBlocksByHash([]string{parentHashStr})
							blocks := <-blocksChan
							if blocks != nil && len(blocks) > 0 {
								// Add the block to any candidate chain that has this header
								n.chainsMu.Lock()
								for _, chain := range n.chains {
									if chain.GetHeader(header.PreviousHash) != nil {
										chain.AddBlock(blocks[0])
									}
								}
								n.chainsMu.Unlock()
							}
						}
					}()
				} else {
					log.Printf("%s\tOrphan header at height %d, parent exists (height %d), not requesting",
						n.Config.NodeID, header.Height, parentHeader.Height)
				}
			}

			return nil
		}
	}

	// Header extends an existing chain
	log.Printf("%s\tHeader %s extends existing chain with tip %s", n.Config.NodeID, blockHash.String(), parentChainTip.String())

	// Add header to the existing chain
	parentChain.AddHeader(header)

	// Add to global headers map for reliable walking
	n.headersMapMu.Lock()
	n.headersMap[blockHash] = header
	n.headersMapMu.Unlock()

	// Try to connect orphan headers that might now have their parent
	n.tryConnectOrphanChildren(blockHash)

	// Remove from orphans if it was there (successful connection)
	var removedFromOrphans bool
	n.orphanHeadersMu.Lock()
	if orphanList, exists := n.orphanHeaders[header.PreviousHash]; exists {
		// Find and remove this specific header from the list
		for i, orphanHeader := range orphanList {
			if orphanHeader.GetHash() == blockHash {
				// Remove this header from the slice
				n.orphanHeaders[header.PreviousHash] = append(orphanList[:i], orphanList[i+1:]...)
				// Clean up empty lists
				if len(n.orphanHeaders[header.PreviousHash]) == 0 {
					delete(n.orphanHeaders, header.PreviousHash)
				}
				removedFromOrphans = true
				break
			}
		}
	}
	n.orphanHeadersMu.Unlock()

	if removedFromOrphans {
		log.Printf("%s\tRemoved header %s from orphans (successfully connected)", n.Config.NodeID, blockHash.String())
	}

	// Update the chain tip in our map (remove old tip, add new tip)
	n.chainsMu.Lock()
	delete(n.chains, parentChainTip)
	n.chains[blockHash] = parentChain
	n.chainsMu.Unlock()

	// Check if this extends the current chain
	n.chainsMu.RLock()
	extendsCurrentChain := (parentChainTip == n.currentChainTip)
	n.chainsMu.RUnlock()

	if extendsCurrentChain {
		log.Printf("%s\tHeader %s extends current chain, fetching block for validation", n.Config.NodeID, blockHash.String())

		// Handle synchronously to avoid race conditions with subsequent headers
		var blocks []*blockchain.Block

		// First check if we have the block locally (from API submission)
		if localBlock := n.GetBlock(blockHash); localBlock != nil {
			blocks = []*blockchain.Block{localBlock}
		} else {
			// Request the block from network
			blockHashStr := base64.StdEncoding.EncodeToString(blockHash[:])
			blocksChan := n.NetworkServer.RequestBlocksByHash([]string{blockHashStr})

			// Wait for block to arrive from network
			blocks = <-blocksChan
		}

		if blocks == nil || len(blocks) == 0 {
			log.Printf("%s\tFailed to fetch block %s for current chain extension", n.Config.NodeID, blockHash.String())
			return nil
		}

		block := blocks[0]

		// Validate block and get new account states
		// Use parentChain instead of re-fetching to avoid race conditions
		newAccountStates, err := blockchain.ValidateBlock(block, parentChain)
		if err != nil {
			log.Printf("%s\tBlock %s failed validation: %v", n.Config.NodeID, blockHash.String(), err)
			// Clean up: remove header from parent chain since validation failed
			// But don't restore to orphans if it's a chain switch error (not a real validation failure)
			restoreToOrphans := true
			if _, isChainSwitch := err.(blockchain.ErrSwitchChain); isChainSwitch {
				restoreToOrphans = false
			}
			n.cleanupFailedHeader(header, blockHash, parentChain, parentChainTip, restoreToOrphans)
			return err
		}

		// Apply block using pre-computed states
		if err := blockchain.ApplyBlock(block, parentChain, newAccountStates); err != nil {
			log.Printf("%s\tFailed to apply block %s: %v", n.Config.NodeID, blockHash.String(), err)
			// Clean up: remove header from parent chain since application failed
			n.cleanupFailedHeader(header, blockHash, parentChain, parentChainTip, true)
			return err
		}

		// Update current chain tip
		n.chainsMu.Lock()
		oldTip := n.currentChainTip
		n.chains[blockHash] = parentChain
		n.currentChainTip = blockHash
		// Remove old tip mapping to avoid stale references
		if oldTip != (blockchain.Hash32{}) && oldTip != blockHash {
			delete(n.chains, oldTip)
		}
		n.chainsMu.Unlock()

		log.Printf("%s\tSuccessfully extended current chain to height %d", n.Config.NodeID, block.Header.Height)

		// Try to connect orphan headers that might now have their parent
		n.tryConnectOrphanChildren(blockHash)
		return nil
	}

	// Handle candidate chain extension
	log.Printf("%s\tHeader %s extends candidate chain, checking for potential reorg (TotalWork: %s)", n.Config.NodeID, blockHash.String(), header.TotalWork)

	// Compare total work between candidate chain and current chain
	n.chainsMu.RLock()
	currentChain := n.chains[n.currentChainTip]
	if currentChain == nil || currentChain.Tip == nil {
		n.chainsMu.RUnlock()
		return nil
	}
	currentWork := currentChain.Tip.Header.TotalWork
	n.chainsMu.RUnlock()

	candidateWork := header.TotalWork
	log.Printf("%s\tWork comparison: candidate=%s, current=%s", n.Config.NodeID, candidateWork, currentWork)
	workComparison := blockchain.CompareWork(candidateWork, currentWork)
	if workComparison <= 0 {
		log.Printf("%s\tCandidate chain has less work, keeping current chain", n.Config.NodeID)
		return nil
	}

	log.Printf("%s\tCandidate chain has more work (candidate: %s, current: %s), performing reorg",
		n.Config.NodeID, candidateWork, currentWork)

	// Fetch all blocks needed for this candidate chain synchronously
	// Find all headers we need to fetch blocks for
	var headersToFetch []blockchain.Hash32
	currentHash := blockHash

	// Walk back from tip to find which blocks we need
	for {
		if block := parentChain.GetBlock(currentHash); block != nil {
			// We already have this block, stop here
			break
		}

		headersToFetch = append([]blockchain.Hash32{currentHash}, headersToFetch...)

		n.headersMapMu.RLock()
		header := n.headersMap[currentHash]
		n.headersMapMu.RUnlock()
		if header == nil {
			break
		}
		currentHash = header.PreviousHash
	}

	// Convert to base64 strings for request
	var hashStrings []string
	for _, hash := range headersToFetch {
		hashStrings = append(hashStrings, base64.StdEncoding.EncodeToString(hash[:]))
	}

	if len(hashStrings) > 0 {
		log.Printf("%s\tFetching %d blocks for candidate chain", n.Config.NodeID, len(hashStrings))
		blocksChan := n.NetworkServer.RequestBlocksByHash(hashStrings)

		blocks := <-blocksChan
		if blocks == nil || len(blocks) != len(hashStrings) {
			log.Printf("%s\tFailed to fetch all blocks for reorg", n.Config.NodeID)
			return nil
		}

		// Sort blocks by height to ensure proper validation order
		sort.Slice(blocks, func(i, j int) bool {
			return blocks[i].Header.Height < blocks[j].Header.Height
		})

		// Create a fresh temporary chain starting from genesis for sequential validation
		tempChain := blockchain.NewChain()

		// Initialize with genesis account states
		genesisStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
		genesisStates[config.FirstUser] = &blockchain.AccountState{
			Balance: 10 * config.AUG,
			Nonce:   0,
		}
		tempChain.SetAccountStates(genesisStates)

		// Sequentially validate each block, only keeping the final state
		var finalAccountStates map[blockchain.PublicKey]*blockchain.AccountState
		for i, block := range blocks {
			newAccountStates, err := blockchain.ValidateBlock(block, tempChain)
			if err != nil {
				log.Printf("%s\tBlock %d failed validation for reorg: %v", n.Config.NodeID, i, err)
				// Don't clean up anything - just abort the reorg attempt
				// The original chains remain intact
				return nil // Fail fast - validation failed
			}

			// Update temp chain with new states and add block for next block validation
			tempChain.SetAccountStates(newAccountStates)
			tempChain.AddBlock(block)
			finalAccountStates = newAccountStates
		}

		// All blocks validated successfully - now perform atomic reorg check and commit
		n.chainsMu.Lock()
		currentChain := n.chains[n.currentChainTip]
		shouldReorg := false

		// Re-check if reorg is still needed (current chain might have grown during validation)
		if currentChain != nil && currentChain.Tip != nil {
			currentWork := currentChain.Tip.Header.TotalWork
			candidateWork := blocks[len(blocks)-1].Header.TotalWork

			if blockchain.CompareWork(candidateWork, currentWork) > 0 {
				shouldReorg = true
				log.Printf("%s\tCandidate chain still has more work, performing atomic reorg", n.Config.NodeID)
			} else {
				log.Printf("%s\tCurrent chain grew during validation, keeping current chain", n.Config.NodeID)
			}
		}

		// Always update candidate chain with validated blocks (don't waste validation work)
		// Add headers to candidate chain
		for _, block := range blocks {
			parentChain.AddHeader(&block.Header)
		}

		// Add full blocks to candidate chain
		for _, block := range blocks {
			parentChain.AddBlock(block)
		}

		// Try to connect orphans for each newly added block
		for _, block := range blocks {
			n.tryConnectOrphanChildren(block.Header.GetHash())
		}

		// Set the final account states
		parentChain.SetAccountStates(finalAccountStates)

		if shouldReorg {
			// Atomic reorg: switch current chain to the candidate chain
			newTipHash := blocks[len(blocks)-1].Header.GetHash()
			n.chains[newTipHash] = parentChain
			n.currentChainTip = newTipHash
			// Old current chain remains in n.chains as a candidate chain

			log.Printf("%s\tSuccessfully performed atomic chain reorganization to height %d",
				n.Config.NodeID, blocks[len(blocks)-1].Header.Height)

			// Clean up old candidate chains
			n.cleanupCandidateChains()
		} else {
			// Update candidate chain tip mapping even if no reorg
			newTipHash := blocks[len(blocks)-1].Header.GetHash()
			n.chains[newTipHash] = parentChain
			delete(n.chains, parentChainTip) // Remove old tip mapping

			log.Printf("%s\tCandidate chain updated but no reorg needed", n.Config.NodeID)
		}

		n.chainsMu.Unlock()
	}

	return nil
}

// tryConnectOrphanChildren attempts to connect orphan headers that are children of the given header
func (n *Node) tryConnectOrphanChildren(parentHash blockchain.Hash32) {
	// Get orphan headers that have this header as their parent
	n.orphanHeadersMu.RLock()
	orphanChildren, exists := n.orphanHeaders[parentHash]
	if !exists || len(orphanChildren) == 0 {
		n.orphanHeadersMu.RUnlock()
		return
	}

	// Make a copy of the orphan list to avoid holding the lock while processing
	childrenToProcess := make([]*blockchain.BlockHeader, len(orphanChildren))
	copy(childrenToProcess, orphanChildren)
	n.orphanHeadersMu.RUnlock()

	// Try to process each orphan child header
	for _, orphanHeader := range childrenToProcess {
		// ProcessHeader will remove from orphans if successful
		go func(header *blockchain.BlockHeader) {
			n.ProcessHeader(header, "orphan-connection")
		}(orphanHeader)
	}
}

// processOrphanHeaders attempts to connect orphan headers to existing chains
func (n *Node) processOrphanHeaders() {
	n.orphanHeadersMu.RLock()
	var orphansToProcess []*blockchain.BlockHeader
	for _, headerList := range n.orphanHeaders {
		orphansToProcess = append(orphansToProcess, headerList...)
	}
	n.orphanHeadersMu.RUnlock()

	// Process each orphan header (ProcessHeader will remove from orphans if successful)
	for _, header := range orphansToProcess {
		n.ProcessHeader(header, "orphan-processing")
	}
}

// cleanupCandidateChains removes candidate chains that are too far behind current chain
func (n *Node) cleanupCandidateChains() {
	// Get current chain height
	currentChain := n.chains[n.currentChainTip]
	if currentChain == nil || currentChain.Tip == nil {
		return
	}

	currentHeight := currentChain.Tip.Header.Height
	minHeight := uint64(0)
	if currentHeight > config.MaxCandidateChainDepthGap {
		minHeight = currentHeight - config.MaxCandidateChainDepthGap
	}

	// Remove candidate chains that are too far behind
	var toRemove []blockchain.Hash32
	var removeInfo []struct {
		hash   blockchain.Hash32
		height uint64
	}

	for tipHash, chain := range n.chains {
		if tipHash == n.currentChainTip {
			continue // Don't remove current chain
		}

		if chain.Tip != nil && chain.Tip.Header.Height < minHeight {
			toRemove = append(toRemove, tipHash)
			removeInfo = append(removeInfo, struct {
				hash   blockchain.Hash32
				height uint64
			}{tipHash, chain.Tip.Header.Height})
		}
	}

	// Remove the stale chains
	for i, tipHash := range toRemove {
		delete(n.chains, tipHash)
		log.Printf("%s\tRemoved stale candidate chain with tip %s (height %d, current height %d)",
			n.Config.NodeID, removeInfo[i].hash.String(), removeInfo[i].height, currentHeight)
	}

	if len(toRemove) > 0 {
		log.Printf("%s\tCleaned up %d stale candidate chains", n.Config.NodeID, len(toRemove))
	}

	// Also cleanup orphan headers that are lower than any candidate chain
	n.cleanupStaleOrphans()
}

// cleanupStaleOrphans removes orphan headers that are lower than any candidate chain
func (n *Node) cleanupStaleOrphans() {
	n.orphanHeadersMu.Lock()
	defer n.orphanHeadersMu.Unlock()

	// Find minimum height across all candidate chains
	minChainHeight := uint64(^uint64(0)) // Start with max uint64
	for _, chain := range n.chains {
		if chain.Tip != nil && chain.Tip.Header.Height < minChainHeight {
			minChainHeight = chain.Tip.Header.Height
		}
	}

	// If no chains have tips, don't remove any orphans
	if minChainHeight == uint64(^uint64(0)) {
		return
	}

	// Remove orphans that are lower than the minimum chain height
	var toRemove []blockchain.Hash32
	for parentHash, headerList := range n.orphanHeaders {
		// Filter out headers that are too old
		var filteredHeaders []*blockchain.BlockHeader
		for _, header := range headerList {
			if header.Height >= minChainHeight {
				filteredHeaders = append(filteredHeaders, header)
			}
		}

		// Update the list or remove it entirely
		if len(filteredHeaders) == 0 {
			toRemove = append(toRemove, parentHash)
		} else {
			n.orphanHeaders[parentHash] = filteredHeaders
		}
	}

	// Remove empty orphan lists
	for _, hash := range toRemove {
		delete(n.orphanHeaders, hash)
	}

	if len(toRemove) > 0 {
		log.Printf("%s\tCleaned up %d stale orphan headers (below minimum chain height %d)",
			n.Config.NodeID, len(toRemove), minChainHeight)
	}
}

// cleanupFailedHeader removes a header from parent chain when validation/application fails
func (n *Node) cleanupFailedHeader(header *blockchain.BlockHeader, blockHash blockchain.Hash32, parentChain *blockchain.Chain, parentChainTip blockchain.Hash32, restoreToOrphans bool) {
	n.chainsMu.Lock()
	defer n.chainsMu.Unlock()

	// Remove header from parent chain
	delete(parentChain.Headers, blockHash)

	// Remove from global headers map since it's invalid
	n.headersMapMu.Lock()
	delete(n.headersMap, blockHash)
	n.headersMapMu.Unlock()

	// Restore original chain mapping (revert the tip update)
	delete(n.chains, blockHash)
	n.chains[parentChainTip] = parentChain

	// Only re-add to orphans if it's a real validation failure (not a chain switch)
	if restoreToOrphans {
		n.orphanHeadersMu.Lock()
		n.orphanHeaders[header.PreviousHash] = append(n.orphanHeaders[header.PreviousHash], header)
		n.orphanHeadersMu.Unlock()
		log.Printf("%s\tCleaned up failed header %s, restored to orphans", n.Config.NodeID, blockHash.String())
	} else {
		log.Printf("%s\tCleaned up failed header %s, not restored to orphans (chain switch)", n.Config.NodeID, blockHash.String())
	}
}

// GetChainHead returns information about the current chain head
func (n *Node) GetChainHead() blockchain.ChainHead {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	if n.currentChainTip == (blockchain.Hash32{}) {
		return blockchain.ChainHead{
			Height:    0,
			Hash:      blockchain.Hash32{},
			TotalWork: "0",
		}
	}

	currentChain := n.chains[n.currentChainTip]
	if currentChain == nil || currentChain.Tip == nil {
		return blockchain.ChainHead{
			Height:    0,
			Hash:      blockchain.Hash32{},
			TotalWork: "0",
		}
	}

	return blockchain.ChainHead{
		Height:    currentChain.Tip.Header.Height,
		Hash:      currentChain.Tip.Header.GetHash(),
		TotalWork: currentChain.Tip.Header.TotalWork,
	}
}

// GetChain returns the current blockchain state (required by NodeAPI interface)
func (n *Node) GetChain() (*blockchain.Chain, error) {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	if n.currentChainTip == (blockchain.Hash32{}) {
		// Return empty chain
		return blockchain.NewChain(), nil
	}

	currentChain := n.chains[n.currentChainTip]
	if currentChain == nil {
		return blockchain.NewChain(), nil
	}

	// Return a copy of the current chain
	chainCopy := &blockchain.Chain{
		Blocks:        make([]*blockchain.Block, len(currentChain.Blocks)),
		BlockIndex:    make(map[blockchain.Hash32]*blockchain.Block),
		Headers:       make(map[blockchain.Hash32]*blockchain.BlockHeader),
		AccountStates: currentChain.GetAccountStates(),
		Tip:           currentChain.Tip,
	}

	copy(chainCopy.Blocks, currentChain.Blocks)
	for k, v := range currentChain.BlockIndex {
		chainCopy.BlockIndex[k] = v
	}
	for k, v := range currentChain.Headers {
		chainCopy.Headers[k] = v
	}

	return chainCopy, nil
}

// GetMempool returns the mempool instance (required by NodeAPI interface)
func (n *Node) GetMempool() *Mempool {
	return n.Mempool
}

// AddOrphanBlock stores an orphan block waiting for its parent
func (n *Node) AddOrphanBlock(block *blockchain.Block, source string, parentHash blockchain.Hash32) {
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

// AddBlockDirectly adds a block directly to the node's blockchain (for testing)
func (n *Node) AddBlockDirectly(block *blockchain.Block) error {
	n.chainsMu.Lock()
	defer n.chainsMu.Unlock()

	blockHash := block.Header.GetHash()

	// Find the appropriate chain to add this block to
	var targetChain *blockchain.Chain
	for _, chain := range n.chains {
		if chain.GetHeader(block.Header.PreviousHash) != nil {
			targetChain = chain
			break
		}
	}

	if targetChain == nil {
		return fmt.Errorf("no chain found for block with parent %s", block.Header.PreviousHash.String())
	}

	// Add header and block to the chain
	targetChain.AddHeader(&block.Header)
	targetChain.AddBlock(block)

	// Add to global headers map
	n.headersMapMu.Lock()
	n.headersMap[blockHash] = &block.Header
	n.headersMapMu.Unlock()

	// Update chain tip mapping
	delete(n.chains, block.Header.PreviousHash)
	n.chains[blockHash] = targetChain

	// Update current chain tip if this extends the current chain
	if block.Header.PreviousHash == n.currentChainTip {
		n.currentChainTip = blockHash
	} else if n.currentChainTip == (blockchain.Hash32{}) {
		// If no current chain tip (genesis scenario), set this as current
		n.currentChainTip = blockHash
	}

	return nil
}

// StoreBlock stores a block in the node's submitted blocks map (used by submit-block API)
func (n *Node) StoreBlock(block *blockchain.Block) error {
	blockHash := block.Header.GetHash()

	n.submittedBlocksMu.Lock()
	defer n.submittedBlocksMu.Unlock()

	// Store the block so it's available for fetching
	n.submittedBlocks[blockHash] = block

	return nil
}

// GetCandidateChains returns a map of all candidate chains (keyed by tip hash)
func (n *Node) GetCandidateChains() map[blockchain.Hash32]*blockchain.Chain {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	result := make(map[blockchain.Hash32]*blockchain.Chain)
	for tipHash, chain := range n.chains {
		result[tipHash] = chain
	}
	return result
}
