package node

import (
	"august/blockchain"
	"log"
)

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
			n.ProcessHeader(header)
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
		n.ProcessHeader(header)
	}
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