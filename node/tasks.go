package node

import (
	"august/blockchain"
	"august/config"
	"log"
)

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