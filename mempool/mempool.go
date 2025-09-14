package mempool

import (
	"august/blockchain"
	"container/heap"
	"sync"
	"time"
)

// MempoolEntry wraps a transaction with metadata
type MempoolEntry struct {
	Transaction blockchain.Transaction
	AddedAt     time.Time
	Fee         uint64 // Cached for sorting
}

// FeeQueue implements a priority queue ordered by transaction fee (highest first)
type FeeQueue []*MempoolEntry

func (pq FeeQueue) Len() int { return len(pq) }

func (pq FeeQueue) Less(i, j int) bool {
	// Higher fee has higher priority (max heap)
	if pq[i].Fee != pq[j].Fee {
		return pq[i].Fee > pq[j].Fee
	}
	// If fees are equal, older transaction has higher priority
	return pq[i].AddedAt.Before(pq[j].AddedAt)
}

func (pq FeeQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *FeeQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*MempoolEntry))
}

func (pq *FeeQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// Config holds mempool configuration
type Config struct {
	MaxSize    int           // Maximum number of transactions (default: 1000)
	MaxExpiry  time.Duration // Maximum age before expiry (default: 7 days)
}

// Mempool manages pending transactions with fee-based prioritization
type Mempool struct {
	mu           sync.RWMutex
	queue        *FeeQueue
	transactions map[string]*MempoolEntry // Hash -> Entry for O(1) lookups
	config       Config
}

// NewMempool creates a new mempool with given configuration
func NewMempool(config Config) *Mempool {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}
	if config.MaxExpiry <= 0 {
		config.MaxExpiry = 7 * 24 * time.Hour // 7 days
	}

	queue := &FeeQueue{}
	heap.Init(queue)

	return &Mempool{
		queue:        queue,
		transactions: make(map[string]*MempoolEntry),
		config:       config,
	}
}

// AddTransaction adds a transaction to the mempool after validation
// Returns true if added, false if rejected (duplicate, invalid, etc.)
func (mp *Mempool) AddTransaction(tx blockchain.Transaction, accountStates map[blockchain.PublicKey]*blockchain.AccountState) bool {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// Generate transaction hash for deduplication
	txHash := blockchain.HashTransaction(&tx)
	hashStr := string(txHash[:])

	// Check if transaction already exists
	if _, exists := mp.transactions[hashStr]; exists {
		return false // Duplicate transaction
	}

	// Validate transaction against current chain state
	if err := blockchain.ValidateTransaction(&tx, accountStates); err != nil {
		return false // Invalid transaction
	}

	entry := &MempoolEntry{
		Transaction: tx,
		AddedAt:     time.Now(),
		Fee:         tx.Fee,
	}

	// If mempool is full, check if we should evict lowest fee transaction
	if len(mp.transactions) >= mp.config.MaxSize {
		// Find the lowest fee transaction (last in max heap)
		if mp.queue.Len() > 0 {
			lowestFeeEntry := (*mp.queue)[mp.queue.Len()-1]

			// Only add if new transaction has higher fee
			if tx.Fee <= lowestFeeEntry.Fee {
				return false // Rejected: fee too low
			}

			// Remove lowest fee transaction
			mp.removeLowestFee()
		}
	}

	// Add transaction to both structures
	heap.Push(mp.queue, entry)
	mp.transactions[hashStr] = entry

	return true
}

// GetTransactions returns the top N transactions ordered by fee (highest first)
func (mp *Mempool) GetTransactions(limit int) []blockchain.Transaction {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	// Clean expired transactions first
	mp.cleanExpiredTransactions()

	var transactions []blockchain.Transaction
	count := limit
	if count > mp.queue.Len() {
		count = mp.queue.Len()
	}

	// Create a copy of the queue for iteration without modifying original
	tempQueue := make(FeeQueue, len(*mp.queue))
	copy(tempQueue, *mp.queue)

	for i := 0; i < count && len(tempQueue) > 0; i++ {
		entry := heap.Pop(&tempQueue).(*MempoolEntry)
		transactions = append(transactions, entry.Transaction)
	}

	return transactions
}

// RemoveTransactions removes the given transactions from the mempool
// Used when transactions are included in a mined block
func (mp *Mempool) RemoveTransactions(transactions []blockchain.Transaction) int {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	removed := 0
	for _, tx := range transactions {
		txHash := blockchain.HashTransaction(&tx)
		hashStr := string(txHash[:])

		if _, exists := mp.transactions[hashStr]; exists {
			delete(mp.transactions, hashStr)
			removed++
		}
	}

	// Rebuild heap after removals (inefficient but simple)
	mp.rebuildHeap()

	return removed
}

// Size returns the current number of transactions in the mempool
func (mp *Mempool) Size() int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return len(mp.transactions)
}

// IsFull returns true if mempool is at capacity
func (mp *Mempool) IsFull() bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return len(mp.transactions) >= mp.config.MaxSize
}

// CleanExpiredTransactions removes transactions older than MaxExpiry
func (mp *Mempool) CleanExpiredTransactions() int {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	return mp.cleanExpiredTransactions()
}

// cleanExpiredTransactions removes expired transactions (must hold lock)
func (mp *Mempool) cleanExpiredTransactions() int {
	now := time.Now()
	removed := 0

	for hash, entry := range mp.transactions {
		if now.Sub(entry.AddedAt) > mp.config.MaxExpiry {
			delete(mp.transactions, hash)
			removed++
		}
	}

	if removed > 0 {
		mp.rebuildHeap()
	}

	return removed
}

// removeLowestFee removes the transaction with lowest fee (must hold lock)
func (mp *Mempool) removeLowestFee() {
	if mp.queue.Len() == 0 {
		return
	}

	// Find the entry with lowest fee (linear search through heap)
	lowestIdx := 0
	lowestFee := (*mp.queue)[0].Fee

	for i, entry := range *mp.queue {
		if entry.Fee < lowestFee || (entry.Fee == lowestFee && entry.AddedAt.After((*mp.queue)[lowestIdx].AddedAt)) {
			lowestIdx = i
			lowestFee = entry.Fee
		}
	}

	// Remove from hash map
	lowestEntry := (*mp.queue)[lowestIdx]
	txHash := blockchain.HashTransaction(&lowestEntry.Transaction)
	hashStr := string(txHash[:])
	delete(mp.transactions, hashStr)

	// Remove from heap by replacing with last element
	lastIdx := mp.queue.Len() - 1
	if lowestIdx != lastIdx {
		// Replace with last element and fix heap
		(*mp.queue)[lowestIdx] = (*mp.queue)[lastIdx]
		*mp.queue = (*mp.queue)[:lastIdx]
		heap.Fix(mp.queue, lowestIdx)
	} else {
		// Just remove the last element
		*mp.queue = (*mp.queue)[:lastIdx]
	}
}

// RevalidateTransactions removes transactions that are no longer valid against current chain state
// This should be called whenever a new block is added to the chain
func (mp *Mempool) RevalidateTransactions(accountStates map[blockchain.PublicKey]*blockchain.AccountState) int {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	removed := 0
	for hash, entry := range mp.transactions {
		// Validate transaction against current chain state
		if err := blockchain.ValidateTransaction(&entry.Transaction, accountStates); err != nil {
			delete(mp.transactions, hash)
			removed++
		}
	}

	if removed > 0 {
		mp.rebuildHeap()
	}

	return removed
}

// rebuildHeap reconstructs the heap from current transactions
func (mp *Mempool) rebuildHeap() {
	newQueue := &FeeQueue{}

	for _, entry := range mp.transactions {
		*newQueue = append(*newQueue, entry)
	}

	heap.Init(newQueue)
	mp.queue = newQueue
}