package mempool

import (
	"august/blockchain"
	"august/types"
	"container/heap"
	"sync"
	"time"
)

// Local type definitions to allow method implementations
type GasPriceQueue []*types.MempoolEntry
type Mempool struct {
	Mu           sync.RWMutex
	Queue        *GasPriceQueue
	Transactions map[string]*types.MempoolEntry
	Config       types.MempoolConfig
}

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
	*pq = append(*pq, x.(*types.MempoolEntry))
}

func (pq *GasPriceQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// NewMempool creates a new mempool with given configuration
func NewMempool(config types.MempoolConfig) *Mempool {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}
	if config.MaxExpiry <= 0 {
		config.MaxExpiry = 7 * 24 * time.Hour // 7 days
	}

	queue := &GasPriceQueue{}
	heap.Init(queue)

	return &Mempool{
		Queue:        queue,
		Transactions: make(map[string]*types.MempoolEntry),
		Config:       config,
	}
}

// AddTransaction adds a transaction to the mempool after validation
// Returns true if added, false if rejected (duplicate, invalid, etc.)
func (mp *Mempool) AddTransaction(tx types.Transaction, accountStates map[types.PublicKey]*types.AccountState) bool {
	mp.Mu.Lock()
	defer mp.Mu.Unlock()

	// Generate transaction hash for deduplication
	txHash := tx.GetHash()
	hashStr := string(txHash[:])

	// Check if transaction already exists
	if _, exists := mp.Transactions[hashStr]; exists {
		return false // Duplicate transaction
	}

	// Validate transaction against current chain state
	if err := blockchain.ValidateTransaction(&tx, accountStates); err != nil {
		return false // Invalid transaction
	}

	entry := &types.MempoolEntry{
		Transaction: tx,
		AddedAt:     time.Now(),
		GasPrice:    tx.GasPrice,
	}

	// If mempool is full, check if we should evict lowest gas price transaction
	if len(mp.Transactions) >= mp.Config.MaxSize {
		// Find the lowest gas price transaction (last in max heap)
		if mp.Queue.Len() > 0 {
			lowestGasPriceEntry := (*mp.Queue)[mp.Queue.Len()-1]

			// Only add if new transaction has higher gas price
			if tx.GasPrice <= lowestGasPriceEntry.GasPrice {
				return false // Rejected: gas price too low
			}

			// Remove lowest gas price transaction
			mp.removeLowestGasPrice()
		}
	}

	// Add transaction to both structures
	heap.Push(mp.Queue, entry)
	mp.Transactions[hashStr] = entry

	return true
}

// GetTransactions returns the top N transactions ordered by fee (highest first)
func (mp *Mempool) GetTransactions(limit int) []types.Transaction {
	// Use write lock since we modify state by cleaning expired transactions
	mp.Mu.Lock()
	defer mp.Mu.Unlock()

	// Clean expired transactions first
	mp.cleanExpiredTransactions()

	var transactions []types.Transaction
	count := limit
	if count > mp.Queue.Len() {
		count = mp.Queue.Len()
	}

	// Create a copy of the queue for iteration without modifying original
	tempQueue := make(GasPriceQueue, len(*mp.Queue))
	copy(tempQueue, *mp.Queue)

	for i := 0; i < count && len(tempQueue) > 0; i++ {
		entry := heap.Pop(&tempQueue).(*types.MempoolEntry)
		transactions = append(transactions, entry.Transaction)
	}

	return transactions
}

// RemoveTransactions removes the given transactions from the mempool
// Used when transactions are included in a mined block
func (mp *Mempool) RemoveTransactions(transactions []types.Transaction) int {
	mp.Mu.Lock()
	defer mp.Mu.Unlock()

	removed := 0
	for _, tx := range transactions {
		txHash := tx.GetHash()
		hashStr := string(txHash[:])

		if _, exists := mp.Transactions[hashStr]; exists {
			delete(mp.Transactions, hashStr)
			removed++
		}
	}

	// Rebuild heap after removals (inefficient but simple)
	mp.rebuildHeap()

	return removed
}

// Size returns the current number of transactions in the mempool
func (mp *Mempool) Size() int {
	mp.Mu.RLock()
	defer mp.Mu.RUnlock()
	return len(mp.Transactions)
}

// IsFull returns true if mempool is at capacity
func (mp *Mempool) IsFull() bool {
	mp.Mu.RLock()
	defer mp.Mu.RUnlock()
	return len(mp.Transactions) >= mp.Config.MaxSize
}

// CleanExpiredTransactions removes transactions older than MaxExpiry
func (mp *Mempool) CleanExpiredTransactions() int {
	mp.Mu.Lock()
	defer mp.Mu.Unlock()
	return mp.cleanExpiredTransactions()
}

// cleanExpiredTransactions removes expired transactions (must hold lock)
func (mp *Mempool) cleanExpiredTransactions() int {
	now := time.Now()
	removed := 0

	for hash, entry := range mp.Transactions {
		if now.Sub(entry.AddedAt) > mp.Config.MaxExpiry {
			delete(mp.Transactions, hash)
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
	if mp.Queue.Len() == 0 {
		return
	}

	// Find the entry with lowest gas price (linear search through heap)
	lowestIdx := 0
	lowestGasPrice := (*mp.Queue)[0].GasPrice

	for i, entry := range *mp.Queue {
		if entry.GasPrice < lowestGasPrice || (entry.GasPrice == lowestGasPrice && entry.AddedAt.After((*mp.Queue)[lowestIdx].AddedAt)) {
			lowestIdx = i
			lowestGasPrice = entry.GasPrice
		}
	}

	// Remove from hash map
	lowestEntry := (*mp.Queue)[lowestIdx]
	txHash := lowestEntry.Transaction.GetHash()
	hashStr := string(txHash[:])
	delete(mp.Transactions, hashStr)

	// Remove from heap by replacing with last element
	lastIdx := mp.Queue.Len() - 1
	if lowestIdx != lastIdx {
		// Replace with last element and fix heap
		(*mp.Queue)[lowestIdx] = (*mp.Queue)[lastIdx]
		*mp.Queue = (*mp.Queue)[:lastIdx]
		heap.Fix(mp.Queue, lowestIdx)
	} else {
		// Just remove the last element
		*mp.Queue = (*mp.Queue)[:lastIdx]
	}
}

// RevalidateTransactions removes transactions that are no longer valid against current chain state
// This should be called whenever a new block is added to the chain
func (mp *Mempool) RevalidateTransactions(accountStates map[types.PublicKey]*types.AccountState) int {
	mp.Mu.Lock()
	defer mp.Mu.Unlock()

	removed := 0
	for hash, entry := range mp.Transactions {
		// Validate transaction against current chain state
		if err := blockchain.ValidateTransaction(&entry.Transaction, accountStates); err != nil {
			delete(mp.Transactions, hash)
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

	for _, entry := range mp.Transactions {
		*newQueue = append(*newQueue, entry)
	}

	heap.Init(newQueue)
	mp.Queue = newQueue
}
