package consensus

import (
	"august/blockchain"
	"august/storage"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CandidateBlock represents an orphan block waiting for its parent
type CandidateBlock struct {
	Block        *Block
	Source       string // Address of peer who sent this block
	ReceivedAt   time.Time
	ParentNeeded Hash32 // Hash of the missing parent block
}

// CandidateChain tracks a competing chain being downloaded and validated
type CandidateChain struct {
	ID             string
	PeerSource     string        // Address of peer advertising this chain
	ChainStore     interface{}   // Isolated storage for candidate blocks
	Headers        []BlockHeader // Headers to validate against
	StartedAt      time.Time
	ExpectedWork   string        // Expected total work when complete
	expectedHeight atomic.Uint64 // Expected final height
	currentHeight  atomic.Uint64 // Current downloaded height
	downloadStatus atomic.Uint64 // 0=downloading, 1=complete, 2=failed
}

// CandidateManager manages candidate blocks and chains
type CandidateManager struct {
	candidateBlocks *sync.Map // map[string]*CandidateBlock
	candidateChains *sync.Map // map[string]*CandidateChain
	store           storage.ChainStore
	mu              sync.RWMutex
}

// NewCandidateManager creates a new candidate manager
func NewCandidateManager(chainStore storage.ChainStore) *CandidateManager {
	return &CandidateManager{
		candidateBlocks: &sync.Map{},
		candidateChains: &sync.Map{},
		store:           chainStore,
	}
}

// AddCandidateBlock stores an orphan block waiting for its parent
func (cm *CandidateManager) AddCandidateBlock(block *blockchain.Block, source string, parentHash [32]byte) {
	blockHash := blockchain.HashBlockHeader(&block.Header)

	candidate := &CandidateBlock{
		Block:        block,
		Source:       source,
		ReceivedAt:   time.Now(),
		ParentNeeded: parentHash,
	}

	cm.candidateBlocks.Store(blockHash, candidate)
}

// CreateCandidateChain creates a new candidate chain for evaluation
func (cm *CandidateManager) CreateCandidateChain(peerAddr string, headers []blockchain.BlockHeader, chainHead *ChainHeadPayload) (*CandidateChain, error) {
	// Generate candidate ID
	candidateID := fmt.Sprintf("%s-%d-%s", peerAddr, chainHead.Height, chainHead.HeadHash[:16])

	// Create candidate chain with isolated storage
	candidateStore := storage.NewMemoryChainStore()

	candidate := &CandidateChain{
		ID:           candidateID,
		PeerSource:   peerAddr,
		ChainStore:   candidateStore,
		Headers:      headers,
		StartedAt:    time.Now(),
		ExpectedWork: chainHead.TotalWork,
	}
	candidate.expectedHeight.Store(chainHead.Height)
	candidate.currentHeight.Store(0)
	candidate.downloadStatus.Store(0) // downloading

	// Store in candidates map
	cm.candidateChains.Store(candidateID, candidate)

	return candidate, nil
}

// GetCandidateChain retrieves a candidate chain by ID
func (cm *CandidateManager) GetCandidateChain(id string) (*CandidateChain, bool) {
	if val, exists := cm.candidateChains.Load(id); exists {
		return val.(*CandidateChain), true
	}
	return nil, false
}

// DeleteCandidateChain removes a candidate chain
func (cm *CandidateManager) DeleteCandidateChain(id string) {
	cm.candidateChains.Delete(id)
}

// ShouldAbortDownload checks if we should abort this download for a better option
func (cm *CandidateManager) ShouldAbortDownload(candidate *CandidateChain) bool {
	bestOtherWork := "0"

	cm.candidateChains.Range(func(key, value interface{}) bool {
		other := value.(*CandidateChain)
		if other.ID != candidate.ID && other.downloadStatus.Load() == 1 { // complete
			if blockchain.CompareWork(other.ExpectedWork, bestOtherWork) > 0 {
				bestOtherWork = other.ExpectedWork
			}
		}
		return true
	})

	// If there's a completed candidate with better work, abort this download
	return blockchain.CompareWork(bestOtherWork, candidate.ExpectedWork) > 0
}

// EvaluateCandidateForPromotion checks if candidate should become active chain
func (cm *CandidateManager) EvaluateCandidateForPromotion(candidate *CandidateChain) error {
	// First pass: Get current chain state without holding lock for long
	var currentWork string
	cm.store.Lock()
	currentChain, err := cm.store.GetChain()
	if err != nil {
		cm.store.Unlock()
		return fmt.Errorf("failed to get current chain: %w", err)
	}

	currentWork = "0"
	if len(currentChain.Blocks) > 0 {
		currentWork = currentChain.Blocks[len(currentChain.Blocks)-1].Header.TotalWork
	}
	cm.store.Unlock()

	// Early exit if candidate doesn't have more work
	if blockchain.CompareWork(candidate.ExpectedWork, currentWork) <= 0 {
		return nil // Not better than current chain
	}

	// Get candidate chain without holding main store lock
	candidateChain, err := candidate.ChainStore.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get candidate chain: %w", err)
	}

	// Expensive validation done WITHOUT holding the store lock
	if err := blockchain.ValidateCompleteChain(candidateChain); err != nil {
		cm.candidateChains.Delete(candidate.ID)
		return fmt.Errorf("candidate failed validation: %w", err)
	}

	// Second pass: Re-acquire lock and double-check before switching
	cm.store.Lock()
	defer cm.store.Unlock()

	// Re-check current state (may have changed during validation)
	currentChain, err = cm.store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to re-check current chain: %w", err)
	}

	newCurrentWork := "0"
	if len(currentChain.Blocks) > 0 {
		newCurrentWork = currentChain.Blocks[len(currentChain.Blocks)-1].Header.TotalWork
	}

	// Final check: still better than current?
	if blockchain.CompareWork(candidate.ExpectedWork, newCurrentWork) > 0 {

		// Atomic switch to new chain (fully validated, no race condition possible under lock)
		if err := cm.store.ReplaceChainUnsafe(candidateChain); err != nil {
			return fmt.Errorf("failed to replace active chain: %w", err)
		}

		// Clean up this candidate
		cm.candidateChains.Delete(candidate.ID)

		return nil // Success - chain was promoted
	}

	// Not better than current chain
	return fmt.Errorf("candidate not better than current chain")
}

// TryConnectCandidateBlocks attempts to connect orphan blocks when new blocks arrive
func (cm *CandidateManager) TryConnectCandidateBlocks(newBlock *blockchain.Block) error {
	newBlockHash := blockchain.HashBlockHeader(&newBlock.Header)
	connected := 0

	// Look for candidate blocks that this block might parent
	cm.candidateBlocks.Range(func(key, value interface{}) bool {
		candidate := value.(*CandidateBlock)

		// Check if this candidate block can now connect
		if candidate.ParentNeeded == newBlockHash {
			// Try to process this candidate block
			if err := cm.processConnectedCandidate(candidate); err == nil {
				// Successfully connected, remove from candidates
				cm.candidateBlocks.Delete(key)
				connected++
			}
		}
		return true
	})

	if connected > 0 {
		// Recursively try to connect more blocks
		return cm.TryConnectCandidateBlocks(newBlock)
	}

	return nil
}

// processConnectedCandidate processes a candidate block that can now connect
func (cm *CandidateManager) processConnectedCandidate(candidate *CandidateBlock) error {
	// Get current chain
	chain, err := cm.store.GetChain()
	if err != nil {
		return fmt.Errorf("failed to get current chain: %w", err)
	}

	// Try to validate and apply the block
	if err := blockchain.ValidateAndApplyBlock(candidate.Block, chain); err != nil {
		return fmt.Errorf("failed to apply candidate block: %w", err)
	}

	// Add to main chain
	if err := cm.store.AddBlock(candidate.Block); err != nil {
		return fmt.Errorf("failed to add block to chain: %w", err)
	}

	return nil
}

// ChainHeadPayload represents a chain head response (moved from networking)
type ChainHeadPayload struct {
	Height    uint64                 `json:"height"`
	HeadHash  string                 `json:"head_hash"`  // Base64 encoded
	TotalWork string                 `json:"total_work"` // Big.Int as string
	Header    blockchain.BlockHeader `json:"header"`     // Full header for validation
}
