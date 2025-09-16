package types

import (
	"sync"
	"time"
)

// MempoolEntry wraps a transaction with metadata
type MempoolEntry struct {
	Transaction Transaction
	AddedAt     time.Time
	GasPrice    uint64 // Cached for sorting (gas price determines priority)
}

// GasPriceQueue implements a priority queue ordered by gas price (highest first)
type GasPriceQueue []*MempoolEntry

// Config holds mempool configuration
type MempoolConfig struct {
	MaxSize   int           // Maximum number of transactions
	MaxExpiry time.Duration // Maximum age before expiry
}

// Mempool manages pending transactions with gas price-based prioritization
type Mempool struct {
	mu           sync.RWMutex
	queue        *GasPriceQueue
	transactions map[string]*MempoolEntry // Hash -> Entry for O(1) lookups
	config       MempoolConfig
}
