package node

import (
	"august/blockchain"
	"august/networking"
	"august/types"
	"encoding/base64"
	"fmt"
	"log"
)

// ProcessBlock attempts to add a block to the main chain, handling orphans
// Returns a completion channel that will be closed when processing completes
// excludePeerAddr: if provided, this peer will be excluded from relay (used when block came from a peer)
func (n *FullNode) ProcessBlock(block *types.Block, excludePeerAddr ...string) <-chan struct{} {
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
		Log("BLOCK", n.Config.NodeID, "Block %x added to main chain", blockHash[:8])

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
					Log("TX", n.Config.NodeID, "Removed %d transactions from mempool (included in block %x)", removed, blockHash[:8])
				}

				// Revalidate remaining transactions against new chain state
				invalidated := n.Mempool.RevalidateTransactions(chainCopy.AccountStates)
				if invalidated > 0 {
					log.Printf("%s\tRemoved %d invalid transactions from mempool after block %x", n.Config.NodeID, invalidated, blockHash[:8])
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

		// Try to connect any candidate blocks that might now be connectible
		if err := n.NetworkServer.GetConsensusManager().TryConnectCandidateBlocks(block); err != nil {
			log.Printf("%s\tError connecting candidate blocks: %v", n.Config.NodeID, err)
		}
	}()

	return complete
}

// ProcessTransaction handles incoming transaction processing
func (n *FullNode) ProcessTransaction(tx *types.Transaction, excludePeerAddr ...string) error {
	if tx == nil {
		return fmt.Errorf("received nil transaction")
	}

	txHash := tx.GetHash()
	Log("TX", n.Config.NodeID, "Processing transaction %x", txHash[:8])

	// Check if already in mempool
	if n.Mempool.HasTransaction(txHash) {
		Log("TX", n.Config.NodeID, "Transaction %x already in mempool", txHash[:8])
		return nil // Not an error, just already processed
	}

	// Validate transaction structure and signature
	if err := blockchain.ValidateTransactionStructure(tx); err != nil {
		Log("TX", n.Config.NodeID, "Transaction %x failed structure validation: %v", txHash[:8], err)
		return fmt.Errorf("invalid transaction structure: %w", err)
	}

	// Add to mempool
	if err := n.Mempool.AddTransaction(tx); err != nil {
		Log("TX", n.Config.NodeID, "Failed to add transaction %x to mempool: %v", txHash[:8], err)
		return fmt.Errorf("mempool rejected transaction: %w", err)
	}

	Log("TX", n.Config.NodeID, "Added transaction %x to mempool", txHash[:8])

	// Relay to other peers (exclude sender)
	if n.NetworkServer != nil {
		go func() {
			<-n.NetworkServer.RelayTransaction(tx, excludePeerAddr...)
		}()
	}

	return nil
}

// ProcessHeaders handles incoming header chain processing
func (n *FullNode) ProcessHeaders(headers []types.BlockHeader, peer string) error {
	if len(headers) == 0 {
		return nil
	}

	Log("HEADERS", n.Config.NodeID, "Processing %d headers from %s", len(headers), peer)

	// Validate header chain
	if !blockchain.ValidateHeaderChain(headers) {
		return fmt.Errorf("invalid header chain from %s", peer)
	}

	// Get our current chain
	ourChain, err := n.Store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get our chain: %w", err)
	}

	ourWork := "0"
	if len(ourChain.Blocks) > 0 {
		ourWork = ourChain.Blocks[len(ourChain.Blocks)-1].Header.TotalWork
	}

	// Check if these headers represent a better chain
	finalHeaderWork := headers[len(headers)-1].TotalWork
	if blockchain.CompareWork(finalHeaderWork, ourWork) <= 0 {
		Log("HEADERS", n.Config.NodeID, "Headers from %s not better than our chain", peer)
		return nil
	}

	Log("HEADERS", n.Config.NodeID, "Headers from %s represent better chain, starting sync", peer)

	// Create hash strings for block requests
	var blockHashes []string
	for _, header := range headers {
		hash := header.GetHash()
		hashString := base64.StdEncoding.EncodeToString(hash[:])
		blockHashes = append(blockHashes, hashString)
	}

	// Request all blocks for this header chain
	if n.NetworkServer != nil {
		go func() {
			blocks, err := n.NetworkServer.RequestBlocksByHash(blockHashes)
			if err != nil {
				Log("HEADERS", n.Config.NodeID, "Failed to download blocks for header chain from %s: %v", peer, err)
				return
			}

			if len(blocks) != len(headers) {
				Log("HEADERS", n.Config.NodeID, "Incomplete block download from %s: got %d blocks for %d headers",
					peer, len(blocks), len(headers))
				return
			}

			Log("HEADERS", n.Config.NodeID, "Downloaded %d blocks for header chain from %s, processing...", len(blocks), peer)

			// Process blocks in order - each ProcessBlock will validate and potentially update the chain
			for i, block := range blocks {
				if block == nil {
					Log("HEADERS", n.Config.NodeID, "Received nil block at index %d from %s", i, peer)
					continue
				}
				// ProcessBlock handles chain validation and reorganization
				<-n.ProcessBlock(block, peer)
			}

			Log("HEADERS", n.Config.NodeID, "Completed processing header chain from %s", peer)
		}()
	}

	return nil
}

// ProcessNewBlockHeader handles a single new block header announcement
func (n *FullNode) ProcessNewBlockHeader(header *types.BlockHeader, peer string) error {
	if header == nil {
		return fmt.Errorf("received nil block header")
	}

	blockHash := header.GetHash()
	Log("HEADER", n.Config.NodeID, "Processing new block header %x (height %d) from %s",
		blockHash[:8], header.Height, peer)

	// Get current chain for evaluation
	ourChain, err := n.Store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get our chain: %w", err)
	}

	// Evaluate if we should request the full block
	evaluation := blockchain.EvaluateBlockHeaderForSync(header, ourChain)

	if evaluation.ShouldRequest {
		Log("HEADER", n.Config.NodeID, "Requesting full block %x: %s", blockHash[:8], evaluation.Reason)

		// Request the full block from the network
		if n.NetworkServer != nil {
			go func() {
				hashString := base64.StdEncoding.EncodeToString(blockHash[:])
				blocks, err := n.NetworkServer.RequestBlocksByHash([]string{hashString})
				if err != nil {
					Log("HEADER", n.Config.NodeID, "Failed to request block %x: %v", blockHash[:8], err)
					return
				}

				if len(blocks) > 0 {
					// Process the received block
					<-n.ProcessBlock(blocks[0], peer)
				}
			}()
		}
	} else {
		Log("HEADER", n.Config.NodeID, "Ignoring header %x: %s", blockHash[:8], evaluation.Reason)
	}

	// Always relay the header to other peers (except sender)
	if n.NetworkServer != nil {
		go func() {
			<-n.NetworkServer.RelayBlockHeader(header, peer)
		}()
	}

	return nil
}

// ProcessChainHead handles chain head information from peers
func (n *FullNode) ProcessChainHead(chainHead *types.ChainHeadPayload, peer string) error {
	if chainHead == nil {
		return fmt.Errorf("received nil chain head")
	}

	Log("CHAINHEAD", n.Config.NodeID, "Processing chain head from %s: height=%d, work=%s",
		peer, chainHead.Height, chainHead.TotalWork)

	// Get our current chain for comparison
	ourChain, err := n.Store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get our chain: %w", err)
	}

	ourHeight := uint64(0)
	if len(ourChain.Blocks) > 0 {
		ourHeight = ourChain.Blocks[len(ourChain.Blocks)-1].Header.Height
	}

	// If peer is significantly ahead, initiate headers-first sync
	if chainHead.Height > ourHeight+1 {
		Log("CHAINHEAD", n.Config.NodeID, "Peer %s is ahead by %d blocks, initiating headers-first sync",
			peer, chainHead.Height-ourHeight)

		// Request headers from peer
		if n.NetworkServer != nil {
			go func() {
				headers, err := n.NetworkServer.RequestHeadersByHeight(1, chainHead.Height)
				if err != nil {
					Log("CHAINHEAD", n.Config.NodeID, "Failed to download headers from %s: %v", peer, err)
					return
				}

				// Process the downloaded headers
				if err := n.ProcessHeaders(headers, peer); err != nil {
					Log("CHAINHEAD", n.Config.NodeID, "Failed to process headers from %s: %v", peer, err)
				}
			}()
		}
	} else if chainHead.Height == ourHeight+1 {
		Log("CHAINHEAD", n.Config.NodeID, "Peer %s has next block, requesting it", peer)

		// Request just the next block
		if n.NetworkServer != nil {
			go func() {
				blocks, err := n.NetworkServer.RequestBlocksByHash([]string{chainHead.HeadHash})
				if err != nil {
					Log("CHAINHEAD", n.Config.NodeID, "Failed to get next block from %s: %v", peer, err)
					return
				}

				if len(blocks) > 0 {
					<-n.ProcessBlock(blocks[0], peer)
				}
			}()
		}
	}

	return nil
}
