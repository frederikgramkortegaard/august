package networking

import (
	"august/config"
	"log"
	"time"
)

// Task represents a periodic task that can be run individually
type Task func()

// TaskScheduler manages all periodic tasks for the server
type TaskScheduler struct {
	server *Server
	tasks  map[string]Task

	// Tickers for different intervals
	cleanupTicker   *time.Ticker
	chainSyncTicker *time.Ticker
	discoveryTicker *time.Ticker
}

// NewTaskScheduler creates a new task scheduler for the server
func (s *Server) NewTaskScheduler() *TaskScheduler {
	ts := &TaskScheduler{
		server: s,
		tasks:  make(map[string]Task),
	}

	// Register all tasks
	ts.tasks["cleanup"] = ts.runCleanupTasks
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

	// Clean up all TTL-based caches
	ts.server.cleanupTTLCaches(now)

	// Clean up dead peers
	removed := ts.server.CleanupDeadPeers()
	if removed > 0 {
		log.Printf(ts.server.config.NodeID+"\t"+"Cleaned up %d dead peers", removed)
	}

	// Log current chain status
	ts.server.logChainStatus()

	log.Printf(ts.server.config.NodeID + "\t" + "Periodic cleanup completed")
}

// periodicDiscovery runs peer discovery at regular intervals
func (ts *TaskScheduler) periodicDiscovery() {
	ticker := time.NewTicker(config.DiscoveryInterval)
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

	currentChain := s.node.GetCurrentChain()

	if currentChain == nil {
		log.Printf(s.config.NodeID + "\t" + "CHAIN STATUS: No blocks")
		return
	}

	blockHash := currentChain.Header.GetHash()
	log.Printf(s.config.NodeID+"\t"+"CHAIN STATUS: Height %d, Head %x, TxCount %d",
		currentChain.Header.Height,
		blockHash[:8],
		len(currentChain.Transactions))
}

// cleanupTTLCaches removes old entries from all TTL-based caches
func (s *Server) cleanupTTLCaches(now time.Time) {
	// Clean up recent headers
	s.recentHeadersMu.Lock()
	headersDeleted := 0
	for hash, addedTime := range s.recentHeaders {
		if now.Sub(addedTime) > config.RecentHeadersTTL {
			delete(s.recentHeaders, hash)
			headersDeleted++
		}
	}
	s.recentHeadersMu.Unlock()

	// Clean up recent blocks
	s.recentBlocksMu.Lock()
	blocksDeleted := 0
	for hash, addedTime := range s.recentBlocks {
		if now.Sub(addedTime) > config.RecentBlocksTTL {
			delete(s.recentBlocks, hash)
			blocksDeleted++
		}
	}
	s.recentBlocksMu.Unlock()

	// Clean up recent transactions
	s.recentTransactionsMu.Lock()
	transactionsDeleted := 0
	for hash, addedTime := range s.recentTransactions {
		if now.Sub(addedTime) > config.RecentTransactionsTTL {
			delete(s.recentTransactions, hash)
			transactionsDeleted++
		}
	}
	s.recentTransactionsMu.Unlock()

	// Log cleanup results
	if headersDeleted > 0 {
		log.Printf(s.config.NodeID+"\t"+"Cleaned up %d old recent headers", headersDeleted)
	}
	if blocksDeleted > 0 {
		log.Printf(s.config.NodeID+"\t"+"Cleaned up %d old recent blocks", blocksDeleted)
	}
	if transactionsDeleted > 0 {
		log.Printf(s.config.NodeID+"\t"+"Cleaned up %d old recent transactions", transactionsDeleted)
	}
}
