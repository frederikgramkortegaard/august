package networking

import (
	"august/config"
	"time"
)

// Task represents a periodic task that can be run individually
type Task func()

// TaskScheduler manages all periodic tasks for the server
type TaskScheduler struct {
	server *Server
	tasks  map[string]Task

	// Tickers for different intervals
	cleanupTicker    *time.Ticker
	chainSyncTicker  *time.Ticker
	discoveryTicker  *time.Ticker
}

// NewTaskScheduler creates a new task scheduler for the server
func (s *Server) NewTaskScheduler() *TaskScheduler {
	ts := &TaskScheduler{
		server: s,
		tasks:  make(map[string]Task),
	}

	// Register all tasks
	ts.tasks["cleanup"] = ts.runCleanupTasks
	ts.tasks["chain-sync"] = ts.runChainSync
	ts.tasks["discovery"] = ts.runDiscovery
	ts.tasks["peer-sharing"] = ts.runPeerSharing

	return ts
}

// StartAllTasks starts all periodic tasks with their default intervals
func (s *Server) StartPeriodicProcesses() error {
	ts := s.NewTaskScheduler()

	// Start unified periodic cleanup system
	ts.cleanupTicker = time.NewTicker(config.CleanupInterval)
	go ts.unifiedPeriodicCleanup()

	// Start chain sync
	go ts.periodicChainSync()

	return nil
}

// RunTask executes a specific task by name
func (s *Server) RunTask(taskName string) {
	ts := s.NewTaskScheduler()
	if task, exists := ts.tasks[taskName]; exists {
		task()
	}
}

// RunAllTasks executes all tasks once (useful for testing)
func (s *Server) RunAllTasks() {
	ts := s.NewTaskScheduler()
	for _, task := range ts.tasks {
		task()
	}
}

// unifiedPeriodicCleanup handles all periodic cleanup tasks in one place
func (ts *TaskScheduler) unifiedPeriodicCleanup() {
	for {
		select {
		case <-ts.cleanupTicker.C:
			ts.runCleanupTasks()
		case <-ts.server.shutdown:
			if ts.cleanupTicker != nil {
				ts.cleanupTicker.Stop()
			}
			return
		}
	}
}

// runCleanupTasks performs all cleanup operations
func (ts *TaskScheduler) runCleanupTasks() {
	now := time.Now()

	// 1. Clean up recent blocks
	ts.server.cleanRecentBlocks(now)

	// 2. Clean up recent transactions
	ts.server.cleanRecentTransactions(now)

	// 3. Clean up dead peers
	removed := ts.server.CleanupDeadPeers()
	if removed > 0 {
		ts.server.logf("Cleaned up %d dead peers", removed)
	}

	// 4. Log current chain status
	ts.server.logChainStatus()

	ts.server.logf("Periodic cleanup completed")
}

// periodicChainSync runs chain synchronization checks
func (ts *TaskScheduler) periodicChainSync() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ts.runChainSync()
		case <-ts.server.shutdown:
			return
		}
	}
}

// runChainSync performs a single chain sync check
func (ts *TaskScheduler) runChainSync() {
	ts.server.checkPeerChains()
}

// periodicDiscovery runs peer discovery at regular intervals
func (ts *TaskScheduler) periodicDiscovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ts.runDiscovery()
	}
}

// runDiscovery performs a single discovery round
func (ts *TaskScheduler) runDiscovery() {
	<-ts.server.RunDiscoveryRound()
}

// runPeerSharing performs peer sharing operations
func (ts *TaskScheduler) runPeerSharing() {
	ts.server.requestPeerSharing()
	<-ts.server.requestPeerSharingAndConnect()
}

// Stop stops all running periodic tasks
func (ts *TaskScheduler) Stop() {
	if ts.cleanupTicker != nil {
		ts.cleanupTicker.Stop()
	}
	if ts.chainSyncTicker != nil {
		ts.chainSyncTicker.Stop()
	}
	if ts.discoveryTicker != nil {
		ts.discoveryTicker.Stop()
	}
}

// Helper functions for tasks

// logChainStatus logs the current state of the blockchain
func (s *Server) logChainStatus() {
	chain, err := s.config.Store.GetChain()
	if err != nil {
		s.logf("Failed to get chain for status: %v", err)
		return
	}

	if len(chain.Blocks) == 0 {
		s.logf("CHAIN STATUS: No blocks")
		return
	}

	latestBlock := chain.Blocks[len(chain.Blocks)-1]
	blockHash := latestBlock.Header.GetHash()
	s.logf("CHAIN STATUS: Height %d, Head %x, TxCount %d",
		latestBlock.Header.Height,
		blockHash[:8],
		len(latestBlock.Transactions))
}

// cleanRecentBlocks removes old entries from the recent blocks cache
func (s *Server) cleanRecentBlocks(now time.Time) {
	s.recentBlocksMu.Lock()
	defer s.recentBlocksMu.Unlock()

	cleaned := 0
	for hash, addedTime := range s.recentBlocks {
		if now.Sub(addedTime) > config.RecentBlocksTTL {
			delete(s.recentBlocks, hash)
			cleaned++
		}
	}

	if cleaned > 0 {
		s.logf("Cleaned up %d old recent blocks", cleaned)
	}
}

// cleanRecentTransactions removes old entries from the recent transactions cache
func (s *Server) cleanRecentTransactions(now time.Time) {
	s.recentTransactionsMu.Lock()
	defer s.recentTransactionsMu.Unlock()

	cleaned := 0
	for hash, addedTime := range s.recentTransactions {
		if now.Sub(addedTime) > config.RecentTransactionsTTL {
			delete(s.recentTransactions, hash)
			cleaned++
		}
	}

	if cleaned > 0 {
		s.logf("Cleaned up %d old recent transactions", cleaned)
	}
}

// checkPeerChains requests chain heads from all connected peers to see if we need to sync
func (s *Server) checkPeerChains() {
	connectedPeers := s.GetConnectedPeers()
	if len(connectedPeers) == 0 {
		return // No peers to sync with
	}

	s.logf("Checking chain heads from %d peers", len(connectedPeers))

	for _, peer := range connectedPeers {
		go func(p *Peer) {
			// Request chain head from peer
			msg, err := NewMessage(MessageTypeRequestChainHead, RequestChainHeadPayload{})
			if err != nil {
				s.logf("Failed to create chain head request for %s: %v", p.Address, err)
				return
			}

			// Use SendNotification since we'll handle the response in handleChainHead
			if err := s.SendNotification(p, msg); err != nil {
				s.logf("Failed to send chain head request to %s: %v", p.Address, err)
			}
		}(peer)
	}
}