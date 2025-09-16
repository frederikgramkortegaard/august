package types

import (
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
	PeerSource     string             // Address of peer advertising this chain
	ChainStore     interface{} // Isolated storage for candidate blocks
	Headers        []BlockHeader      // Headers to validate against
	StartedAt      time.Time
	ExpectedWork   string        // Expected total work when complete
	expectedHeight atomic.Uint64 // Expected final height
	currentHeight  atomic.Uint64 // Current downloaded height
	downloadStatus atomic.Uint64 // 0=downloading, 1=complete, 2=failed
}
