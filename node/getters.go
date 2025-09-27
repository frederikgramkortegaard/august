package node

import "august/blockchain"

// GetCurrentChain returns the current chain's tip block
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

// GetBlock retrieves a block by its hash, checking submitted blocks first then all chains
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

// GetBlocks retrieves multiple blocks by their hashes
func (n *Node) GetBlocks(hashes []blockchain.Hash32) []*blockchain.Block {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	var blocks []*blockchain.Block

	for _, hash := range hashes {
		if block := n.GetBlock(hash); block != nil {
			blocks = append(blocks, block)
			continue
		}
	}
	return blocks
}

// GetHeader retrieves a block header by its hash
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

// GetHeaders retrieves multiple headers by their hashes
func (n *Node) GetHeaders(hashes []blockchain.Hash32) []*blockchain.BlockHeader {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	var headers []*blockchain.BlockHeader

	for _, hash := range hashes {
		// Search through all chains for the header
		for _, chain := range n.chains {
			if header := chain.GetHeader(hash); header != nil {
				headers = append(headers, header)
				continue
			}
		}
	}
	return headers
}

// GetBlockAtHeight retrieves a block at a specific height from the current chain
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
