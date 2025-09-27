package node

import (
	"august/blockchain"
	"august/config"
	"august/libnet"
	"august/libnet/protobuf"
	"fmt"
	"github.com/google/uuid"
	"log"
	"sort"
	"time"
)

func (n *Node) checkHeaderExists(blockHash blockchain.Hash32) bool {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	for _, chain := range n.chains {
		if chain.GetHeader(blockHash) != nil {
			return true
		}
	}
	return false
}

type parentChainResult struct {
	chain *blockchain.Chain
	tip   blockchain.Hash32
}

func (n *Node) findParentChain(header *blockchain.BlockHeader) *parentChainResult {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	for tipHash, chain := range n.chains {
		if tipHash == header.PreviousHash {
			if parentHeader := chain.GetHeader(header.PreviousHash); parentHeader != nil {
				if err := blockchain.ValidateHeaderTotalWork(header, parentHeader.TotalWork); err != nil {
					return nil
				}
				return &parentChainResult{
					chain: chain,
					tip:   tipHash,
				}
			}
		}
	}
	return nil
}

type parentBlockResult struct {
	block       *blockchain.Block
	sourceChain *blockchain.Chain
}

func (n *Node) findParentBlock(header *blockchain.BlockHeader) *parentBlockResult {
	n.chainsMu.RLock()
	defer n.chainsMu.RUnlock()

	for _, chain := range n.chains {
		if block := chain.GetBlock(header.PreviousHash); block != nil {
			return &parentBlockResult{
				block:       block,
				sourceChain: chain,
			}
		}
	}
	return nil
}

func (n *Node) buildChainFromBlocks(blocks []*blockchain.Block) (*blockchain.Chain, error) {
	newChain := blockchain.NewChain()

	for _, block := range blocks {
		newAccountStates, err := blockchain.ValidateBlock(block, newChain)
		if err != nil {
			log.Printf("%s\tFailed to validate block in chain creation: %v", n.Config.NodeID, err)
			return nil, fmt.Errorf("failed to build chain: %w", err)
		}

		if err := blockchain.ApplyBlock(block, newChain, newAccountStates); err != nil {
			log.Printf("%s\tFailed to apply block in chain creation: %v", n.Config.NodeID, err)
			return nil, fmt.Errorf("failed to build chain: %w", err)
		}

		n.tryConnectOrphanChildren(block.Header.GetHash())
	}

	return newChain, nil
}

func (n *Node) fetchMissingBlocksForFork(header *blockchain.BlockHeader, missingBlocks []blockchain.Hash32, sourceChain *blockchain.Chain) {
	go func() {
		messageID := uuid.New().String()
		ch := n.PeerService.AddMessageToPending(messageID)
		n.PeerService.RPC.RequestBlocks(messageID, missingBlocks)

		select {
		case <-ch:
		case <-time.After(2 * time.Minute):
			log.Printf("%s\tTimeout waiting for blocks response", n.Config.NodeID)
			return
		}

		data := n.PeerService.GetDataFromMessage(messageID)
		if len(data) == 0 {
			log.Printf("%s\tFailed to fetch blocks for new chain creation", n.Config.NodeID)
			return
		}

		blocksMap := make(map[blockchain.Hash32]*blockchain.Block)
		for _, msg := range data {
			resp := msg.(*protobuf.BlocksResponse)
			for _, protoBlock := range resp.BlockData {
				block := libnet.ProtoBlockToBlock(protoBlock)
				blocksMap[block.GetHash()] = block
			}
		}

		n.chainsMu.Lock()
		for _, block := range blocksMap {
			sourceChain.AddBlock(block)
		}
		n.chainsMu.Unlock()

		n.ProcessHeader(header)
	}()
}

func (n *Node) handleForkCreation(header *blockchain.BlockHeader, parentBlock *blockchain.Block, sourceChain *blockchain.Chain) (*parentChainResult, error) {
	blockHash := header.GetHash()
	log.Printf("%s\tHeader %s creates new candidate chain (fork from block %s)",
		n.Config.NodeID, blockHash.String(), header.PreviousHash.String())

	var missingBlocks []blockchain.Hash32
	var blocksToApply []*blockchain.Block
	currentHash := parentBlock.Header.GetHash()

	for currentHash != blockchain.GenesisBlock.Header.GetHash() {
		h := n.GetHeader(currentHash)
		if h == nil {
			log.Printf("%s\tMissing header %s during traversal", n.Config.NodeID, currentHash.String()[:8])
			break
		}

		block := n.GetBlock(currentHash)
		if block == nil {
			missingBlocks = append(missingBlocks, currentHash)
		} else {
			blocksToApply = append([]*blockchain.Block{block}, blocksToApply...)
		}

		currentHash = h.PreviousHash
	}

	if len(missingBlocks) > 0 {
		log.Printf("%s\tMissing %d blocks for new chain creation, fetching blocks", n.Config.NodeID, len(missingBlocks))
		n.fetchMissingBlocksForFork(header, missingBlocks, sourceChain)
		return nil, nil
	}

	newChain, err := n.buildChainFromBlocks(blocksToApply)
	if err != nil {
		return nil, err
	}

	parentHash := parentBlock.Header.GetHash()
	n.chains[parentHash] = newChain

	log.Printf("%s\tCreated new candidate chain with tip %s, continuing with header processing",
		n.Config.NodeID, parentHash.String())

	return &parentChainResult{
		chain: newChain,
		tip:   parentHash,
	}, nil
}

func (n *Node) handleOrphanHeader(header *blockchain.BlockHeader) error {
	blockHash := header.GetHash()
	log.Printf("%s\tHeader %s is orphan (parent not found), adding to orphan headers", n.Config.NodeID, blockHash.String())

	n.orphanHeadersMu.Lock()
	n.orphanHeaders[header.PreviousHash] = append(n.orphanHeaders[header.PreviousHash], header)
	n.orphanHeadersMu.Unlock()

	parentHeader := n.GetHeader(header.PreviousHash)
	if parentHeader == nil {
		log.Printf("%s\tOrphan header at height %d, requesting parent header and block to find common ancestor",
			n.Config.NodeID, header.Height)
		n.fetchParentForOrphan(header)
	}

	return nil
}

func (n *Node) fetchParentForOrphan(header *blockchain.BlockHeader) {
	go func() {
		// Request parent header first
		headerMessageID := uuid.New().String()
		headerCh := n.PeerService.AddMessageToPending(headerMessageID)
		n.PeerService.RPC.RequestHeaders(headerMessageID, []blockchain.Hash32{header.PreviousHash})

		select {
		case <-headerCh:
		case <-time.After(2 * time.Minute):
			log.Printf("%s\tTimeout waiting for parent header", n.Config.NodeID)
			return
		}

		headerData := n.PeerService.GetDataFromMessage(headerMessageID)
		if len(headerData) == 0 {
			return
		}

		// Process parent header
		resp := headerData[0].(*protobuf.HeadersResponse)
		if len(resp.HeaderData) > 0 {
			parentHeader := libnet.ProtoHeaderToBlockHeader(resp.HeaderData[0])
			n.ProcessHeader(parentHeader)

			// Request parent block
			blockMessageID := uuid.New().String()
			blockCh := n.PeerService.AddMessageToPending(blockMessageID)
			n.PeerService.RPC.RequestBlocks(blockMessageID, []blockchain.Hash32{header.PreviousHash})

			select {
			case <-blockCh:
			case <-time.After(2 * time.Minute):
				log.Printf("%s\tTimeout waiting for parent block", n.Config.NodeID)
				return
			}

			blockData := n.PeerService.GetDataFromMessage(blockMessageID)
			if len(blockData) > 0 {
				blockResp := blockData[0].(*protobuf.BlocksResponse)
				if len(blockResp.BlockData) > 0 {
					block := libnet.ProtoBlockToBlock(blockResp.BlockData[0])
					n.chainsMu.Lock()
					for _, chain := range n.chains {
						if chain.GetHeader(header.PreviousHash) != nil {
							chain.AddBlock(block)
						}
					}
					n.chainsMu.Unlock()
				}
			}
		}
	}()
}
func (n *Node) handleChainExtension(header *blockchain.BlockHeader, parentChain *blockchain.Chain, parentChainTip blockchain.Hash32) error {
	blockHash := header.GetHash()
	log.Printf("%s\tHeader %s extends existing chain with tip %s", n.Config.NodeID, blockHash.String(), parentChainTip.String())

	parentChain.AddHeader(header)

	n.headersMapMu.Lock()
	n.headersMap[blockHash] = header
	n.headersMapMu.Unlock()

	n.tryConnectOrphanChildren(blockHash)

	var removedFromOrphans bool
	n.orphanHeadersMu.Lock()
	if orphanList, exists := n.orphanHeaders[header.PreviousHash]; exists {
		for i, orphanHeader := range orphanList {
			if orphanHeader.GetHash() == blockHash {
				n.orphanHeaders[header.PreviousHash] = append(orphanList[:i], orphanList[i+1:]...)
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

	n.chainsMu.Lock()
	delete(n.chains, parentChainTip)
	n.chains[blockHash] = parentChain
	n.chainsMu.Unlock()

	n.chainsMu.RLock()
	extendsCurrentChain := (parentChainTip == n.currentChainTip)
	n.chainsMu.RUnlock()

	if extendsCurrentChain {
		return n.handleCurrentChainExtension(header, blockHash, parentChain, parentChainTip)
	}

	return n.handleCandidateChainExtension(header, blockHash, parentChain, parentChainTip)
}

func (n *Node) handleCurrentChainExtension(header *blockchain.BlockHeader, blockHash blockchain.Hash32, parentChain *blockchain.Chain, parentChainTip blockchain.Hash32) error {
	log.Printf("%s\tHeader %s extends current chain, fetching block for validation", n.Config.NodeID, blockHash.String())

	var blocks []*blockchain.Block

	if localBlock := n.GetBlock(blockHash); localBlock != nil {
		blocks = []*blockchain.Block{localBlock}
	} else {
		messageID := uuid.New().String()
		ch := n.PeerService.AddMessageToPending(messageID)
		n.PeerService.RPC.RequestBlocks(messageID, []blockchain.Hash32{blockHash})

		select {
		case <-ch:
		case <-time.After(2 * time.Minute):
			log.Printf("%s\tTimeout waiting for block %s", n.Config.NodeID, blockHash.String()[:8])
			return nil
		}

		data := n.PeerService.GetDataFromMessage(messageID)
		if len(data) > 0 {
			blocksMap := make(map[blockchain.Hash32]*blockchain.Block)
			for _, msg := range data {
				resp := msg.(*protobuf.BlocksResponse)
				for _, protoBlock := range resp.BlockData {
					block := libnet.ProtoBlockToBlock(protoBlock)
					blocksMap[block.GetHash()] = block
				}
			}
			for _, block := range blocksMap {
				blocks = append(blocks, block)
			}
		}
	}

	if blocks == nil || len(blocks) == 0 {
		log.Printf("%s\tFailed to fetch block %s for current chain extension", n.Config.NodeID, blockHash.String())
		return nil
	}

	block := blocks[0]

	newAccountStates, err := blockchain.ValidateBlock(block, parentChain)
	if err != nil {
		log.Printf("%s\tBlock %s failed validation: %v", n.Config.NodeID, blockHash.String(), err)
		restoreToOrphans := true
		if _, isChainSwitch := err.(blockchain.ErrSwitchChain); isChainSwitch {
			restoreToOrphans = false
		}
		n.cleanupFailedHeader(header, blockHash, parentChain, parentChainTip, restoreToOrphans)
		return err
	}

	if err := blockchain.ApplyBlock(block, parentChain, newAccountStates); err != nil {
		log.Printf("%s\tFailed to apply block %s: %v", n.Config.NodeID, blockHash.String(), err)
		n.cleanupFailedHeader(header, blockHash, parentChain, parentChainTip, true)
		return err
	}

	n.blocksMapMu.Lock()
	n.blocksMap[blockHash] = block
	n.blocksMapMu.Unlock()

	n.chainsMu.Lock()
	oldTip := n.currentChainTip
	n.chains[blockHash] = parentChain
	n.currentChainTip = blockHash
	if oldTip != (blockchain.Hash32{}) && oldTip != blockHash {
		delete(n.chains, oldTip)
	}
	n.chainsMu.Unlock()

	log.Printf("%s\tSuccessfully extended current chain to height %d", n.Config.NodeID, block.Header.Height)

	n.tryConnectOrphanChildren(blockHash)
	return nil
}

func (n *Node) handleCandidateChainExtension(header *blockchain.BlockHeader, blockHash blockchain.Hash32, parentChain *blockchain.Chain, parentChainTip blockchain.Hash32) error {
	log.Printf("%s\tHeader %s extends candidate chain, checking for potential reorg (TotalWork: %s)", n.Config.NodeID, blockHash.String(), header.TotalWork)

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

	var headersToFetch []blockchain.Hash32
	currentHash := blockHash

	for {
		if block := parentChain.GetBlock(currentHash); block != nil {
			break
		}

		headersToFetch = append([]blockchain.Hash32{currentHash}, headersToFetch...)

		n.headersMapMu.RLock()
		h := n.headersMap[currentHash]
		n.headersMapMu.RUnlock()
		if h == nil {
			break
		}
		currentHash = h.PreviousHash
	}

	if len(headersToFetch) > 0 {
		log.Printf("%s\tFetching %d blocks for candidate chain", n.Config.NodeID, len(headersToFetch))

		messageID := uuid.New().String()
		ch := n.PeerService.AddMessageToPending(messageID)
		n.PeerService.RPC.RequestBlocks(messageID, headersToFetch)

		select {
		case <-ch:
		case <-time.After(2 * time.Minute):
			log.Printf("%s\tTimeout waiting for blocks for reorg", n.Config.NodeID)
			return nil
		}

		data := n.PeerService.GetDataFromMessage(messageID)
		if len(data) == 0 {
			log.Printf("%s\tFailed to fetch all blocks for reorg", n.Config.NodeID)
			return nil
		}

		blocksMap := make(map[blockchain.Hash32]*blockchain.Block)
		for _, msg := range data {
			resp := msg.(*protobuf.BlocksResponse)
			for _, protoBlock := range resp.BlockData {
				block := libnet.ProtoBlockToBlock(protoBlock)
				blocksMap[block.GetHash()] = block
			}
		}

		var blocks []*blockchain.Block
		for _, block := range blocksMap {
			blocks = append(blocks, block)
		}

		sort.Slice(blocks, func(i, j int) bool {
			return blocks[i].Header.Height < blocks[j].Header.Height
		})

		tempChain := blockchain.NewChain()

		genesisStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
		genesisStates[config.FirstUser] = &blockchain.AccountState{
			Balance: 10 * config.AUG,
			Nonce:   0,
		}
		tempChain.SetAccountStates(genesisStates)

		var finalAccountStates map[blockchain.PublicKey]*blockchain.AccountState
		for i, block := range blocks {
			newAccountStates, err := blockchain.ValidateBlock(block, tempChain)
			if err != nil {
				log.Printf("%s\tBlock %d failed validation for reorg: %v", n.Config.NodeID, i, err)
				return nil
			}

			tempChain.SetAccountStates(newAccountStates)
			tempChain.AddBlock(block)
			finalAccountStates = newAccountStates
		}

		n.chainsMu.Lock()
		currentChain := n.chains[n.currentChainTip]
		shouldReorg := false

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

		// Add genesis to parent chain first
		parentChain.AddHeader(&blockchain.GenesisBlock.Header)
		parentChain.AddBlock(blockchain.GenesisBlock)

		n.headersMapMu.Lock()
		for _, block := range blocks {
			parentChain.AddHeader(&block.Header)
			n.headersMap[block.Header.GetHash()] = &block.Header
		}
		n.headersMapMu.Unlock()

		n.blocksMapMu.Lock()
		for _, block := range blocks {
			parentChain.AddBlock(block)
			n.blocksMap[block.Header.GetHash()] = block
		}
		n.blocksMapMu.Unlock()

		for _, block := range blocks {
			n.tryConnectOrphanChildren(block.Header.GetHash())
		}

		parentChain.SetAccountStates(finalAccountStates)

		if shouldReorg {
			newTipHash := blocks[len(blocks)-1].Header.GetHash()
			n.chains[newTipHash] = parentChain
			n.currentChainTip = newTipHash

			log.Printf("%s\tSuccessfully performed atomic chain reorganization to height %d",
				n.Config.NodeID, blocks[len(blocks)-1].Header.Height)

			n.cleanupCandidateChains()
		} else {
			newTipHash := blocks[len(blocks)-1].Header.GetHash()
			n.chains[newTipHash] = parentChain
			delete(n.chains, parentChainTip)

			log.Printf("%s\tCandidate chain updated but no reorg needed", n.Config.NodeID)
		}

		n.chainsMu.Unlock()
	}

	return nil
}
