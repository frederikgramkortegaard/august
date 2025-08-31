package processing

import (
	"fmt"
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"gocuria/p2p"
	"log"
)

// BlockProcessor handles block processing, validation, and orphan management
type BlockProcessor struct {
	store      store.ChainStore
	orphanPool map[blockchain.Hash32]*blockchain.Block
	p2pServer  *p2p.Server // Optional: for relaying blocks
}

// NewBlockProcessor creates a new block processor
func NewBlockProcessor(chainStore store.ChainStore) *BlockProcessor {
	return &BlockProcessor{
		store:      chainStore,
		orphanPool: make(map[blockchain.Hash32]*blockchain.Block),
	}
}

// SetP2PServer sets the P2P server for block relaying (optional)
func (bp *BlockProcessor) SetP2PServer(server *p2p.Server) {
	bp.p2pServer = server
}

// ProcessBlock attempts to add a block to the main chain, handling orphans
func (bp *BlockProcessor) ProcessBlock(block *blockchain.Block) error {
	blockHash := blockchain.HashBlockHeader(&block.Header)
	
	// Get current chain for validation
	chain, err := bp.store.GetChain()
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
			bp.orphanPool[blockHash] = block
			
			// TODO: Request missing parent block from peers
			log.Printf("Need to request parent block %x from peers", missingParentErr.Hash[:8])
			return nil
		}
		
		// Other validation errors
		return fmt.Errorf("block validation failed: %w", err)
	}
	
	// Block validation succeeded, now persist it to the store
	if err := bp.store.AddBlock(block); err != nil {
		return fmt.Errorf("failed to persist block to store: %w", err)
	}
	
	// Block successfully added to main chain
	log.Printf("Block %x added to main chain", blockHash[:8])
	
	// Relay the block to connected peers (if we have a P2P server)
	if bp.p2pServer != nil {
		bp.p2pServer.RelayBlock(block)
	}
	
	// Try to connect any orphan blocks that might now be connectible
	bp.tryConnectOrphans()
	
	return nil
}

// tryConnectOrphans attempts to connect orphan blocks to the main chain
func (bp *BlockProcessor) tryConnectOrphans() {
	connected := true
	
	// Keep trying until no more orphans can be connected
	for connected {
		connected = false
		
		// Get current chain state for each attempt
		chain, err := bp.store.GetChain()
		if err != nil {
			log.Printf("Failed to get chain for orphan connection: %v", err)
			return
		}
		
		// Check each orphan block
		for orphanHash, orphanBlock := range bp.orphanPool {
			// Try to validate and add this orphan block
			if err := blockchain.ValidateAndApplyBlock(orphanBlock, chain); err == nil {
				// Validation succeeded, now persist to store
				if err := bp.store.AddBlock(orphanBlock); err != nil {
					log.Printf("Failed to persist orphan block %x to store: %v", orphanHash[:8], err)
					continue
				}
				
				// Successfully connected!
				log.Printf("Connected orphan block %x to main chain", orphanHash[:8])
				
				// Remove from orphan pool
				delete(bp.orphanPool, orphanHash)
				connected = true
				
				// Don't continue iterating as we modified the map
				break
			}
		}
	}
	
	if len(bp.orphanPool) > 0 {
		log.Printf("Still have %d orphan blocks waiting for parents", len(bp.orphanPool))
	}
}

// GetOrphanCount returns the number of orphan blocks waiting for parents
func (bp *BlockProcessor) GetOrphanCount() int {
	return len(bp.orphanPool)
}
