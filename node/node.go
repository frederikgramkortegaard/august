package node

import (
	"august/blockchain"
	"august/libnet"
	"august/networking"
	"context"
	"fmt"
	"log"
	"sync"
)

// NodeConfig holds all configuration for a full node
type NodeConfig struct {
	Port      int
	NodeID    string
	SeedPeers []string
	QueryPort int // HTTP port for query API and miners (optional, 0 means disabled)

}

// Node orchestrates Peer Discovery and the rest of networking stuff
type Node struct {

	// Configuration
	Config NodeConfig

	// Components (each package handles its own concern)
	NetworkServer *networking.Server  // Network message handling
	PeerService   *libnet.PeerService // Handles P2P Networking //@NOTE : Replaces NetworkServer
	Mempool       *Mempool            // Transaction pool

	// Chain Management
	chains          map[blockchain.Hash32]*blockchain.Chain // keyed by tip hash
	chainsMu        sync.RWMutex
	currentChainTip blockchain.Hash32 // points to which chain is current

	// Global header index for reliable header walking across chains
	headersMap   map[blockchain.Hash32]*blockchain.BlockHeader
	headersMapMu sync.RWMutex

	// Global block index for efficient block lookup across chains
	blocksMap   map[blockchain.Hash32]*blockchain.Block
	blocksMapMu sync.RWMutex

	// Orphan header management (keyed by PreviousHash - the parent they're waiting for)
	orphanHeaders   map[blockchain.Hash32][]*blockchain.BlockHeader
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
func (n *Node) ProcessHeader(header *blockchain.BlockHeader) error {
	n.headerProcessingMu.Lock()
	defer n.headerProcessingMu.Unlock()

	blockHash := header.GetHash()
	log.Printf("%s\tProcessing header %s (height %d)", n.Config.NodeID, blockHash.String()[:8], header.Height)

	if err := blockchain.ValidateHeaderStructure(header); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	if n.checkHeaderExists(blockHash) {
		return nil
	}

	parentChainResult := n.findParentChain(header)
	if parentChainResult != nil {
		return n.handleChainExtension(header, parentChainResult.chain, parentChainResult.tip)
	}

	parentBlockResult := n.findParentBlock(header)
	if parentBlockResult != nil {
		result, err := n.handleForkCreation(header, parentBlockResult.block, parentBlockResult.sourceChain)
		if err != nil {
			return err
		}
		if result == nil {
			return nil
		}
		return n.handleChainExtension(header, result.chain, result.tip)
	}

	return n.handleOrphanHeader(header)
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

	// Add to global headers and blocks maps
	n.headersMapMu.Lock()
	n.headersMap[blockHash] = &block.Header
	n.headersMapMu.Unlock()

	n.blocksMapMu.Lock()
	n.blocksMap[blockHash] = block
	n.blocksMapMu.Unlock()

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
