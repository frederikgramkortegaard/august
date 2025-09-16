package node

import (
	"august/blockchain"
	"august/config"
	"container/heap"
	"sync"
	"time"
)

// MempoolEntry wraps a transaction with metadata
type MempoolEntry struct {
	Transaction blockchain.Transaction
	AddedAt     time.Time
	GasPrice    uint64 // Cached for sorting (gas price determines priority)
}

// GasPriceQueue implements a priority queue ordered by gas price (highest first)
type GasPriceQueue []*MempoolEntry

func (pq GasPriceQueue) Len() int { return len(pq) }

func (pq GasPriceQueue) Less(i, j int) bool {
	// Higher gas price has higher priority (max heap)
	if pq[i].GasPrice != pq[j].GasPrice {
		return pq[i].GasPrice > pq[j].GasPrice
	}
	// If gas prices are equal, older transaction has higher priority
	return pq[i].AddedAt.Before(pq[j].AddedAt)
}

func (pq GasPriceQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *GasPriceQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*MempoolEntry))
}

func (pq *GasPriceQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}


// Mempool manages pending transactions with gas price-based prioritization
type Mempool struct {
	mu           sync.RWMutex
	queue        *GasPriceQueue
	transactions map[string]*MempoolEntry // Hash -> Entry for O(1) lookups
}

// NewMempool creates a new mempool
func NewMempool() *Mempool {
	queue := &GasPriceQueue{}
	heap.Init(queue)

	return &Mempool{
		queue:        queue,
		transactions: make(map[string]*MempoolEntry),
	}
}

// AddTransaction adds a transaction to the mempool after validation
// Returns true if added, false if rejected (duplicate, invalid, etc.)
func (mp *Mempool) AddTransaction(tx blockchain.Transaction, accountStates map[blockchain.PublicKey]*blockchain.AccountState) bool {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// Generate transaction hash for deduplication
	txHash := tx.GetHash()
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
		GasPrice:    tx.GasPrice,
	}

	// If mempool is full, check if we should evict lowest gas price transaction
	if len(mp.transactions) >= config.MempoolMaxSize {
		// Find the lowest gas price transaction (last in max heap)
		if mp.queue.Len() > 0 {
			lowestGasPriceEntry := (*mp.queue)[mp.queue.Len()-1]

			// Only add if new transaction has higher gas price
			if tx.GasPrice <= lowestGasPriceEntry.GasPrice {
				return false // Rejected: gas price too low
			}

			// Remove lowest gas price transaction
			mp.removeLowestGasPrice()
		}
	}

	// Add transaction to both structures
	heap.Push(mp.queue, entry)
	mp.transactions[hashStr] = entry

	return true
}

// GetTransactions returns the top N transactions ordered by fee (highest first)
func (mp *Mempool) GetTransactions(limit int) []blockchain.Transaction {
	// Use write lock since we modify state by cleaning expired transactions
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// Clean expired transactions first
	mp.cleanExpiredTransactions()

	var transactions []blockchain.Transaction
	count := limit
	if count > mp.queue.Len() {
		count = mp.queue.Len()
	}

	// Create a copy of the queue for iteration without modifying original
	tempQueue := make(GasPriceQueue, len(*mp.queue))
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
		txHash := tx.GetHash()
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
	return len(mp.transactions) >= config.MempoolMaxSize
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
		if now.Sub(entry.AddedAt) > config.MempoolMaxExpiry {
			delete(mp.transactions, hash)
			removed++
		}
	}

	if removed > 0 {
		mp.rebuildHeap()
	}

	return removed
}

// removeLowestGasPrice removes the transaction with lowest gas price (must hold lock)
func (mp *Mempool) removeLowestGasPrice() {
	if mp.queue.Len() == 0 {
		return
	}

	// Find the entry with lowest gas price (linear search through heap)
	lowestIdx := 0
	lowestGasPrice := (*mp.queue)[0].GasPrice

	for i, entry := range *mp.queue {
		if entry.GasPrice < lowestGasPrice || (entry.GasPrice == lowestGasPrice && entry.AddedAt.After((*mp.queue)[lowestIdx].AddedAt)) {
			lowestIdx = i
			lowestGasPrice = entry.GasPrice
		}
	}

	// Remove from hash map
	lowestEntry := (*mp.queue)[lowestIdx]
	txHash := lowestEntry.Transaction.GetHash()
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
	newQueue := &GasPriceQueue{}

	for _, entry := range mp.transactions {
		*newQueue = append(*newQueue, entry)
	}

	heap.Init(newQueue)
	mp.queue = newQueue
}