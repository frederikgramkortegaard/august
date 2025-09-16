package networking

import (
	"august/config"
	"fmt"
	"time"
)

// cleanupDeadPeers removes disconnected peers from the peer list
func (n *Network) cleanupDeadPeers() int {
	n.peersMu.Lock()
	defer n.peersMu.Unlock()

	removed := 0
	for addr, peer := range n.peers {
		if peer.Status == PeerDisconnected || peer.Status == PeerFailed {
			delete(n.peers, addr)
			removed++
		}
	}
	return removed
}

// cleanupExpiredRecentBlocks removes blocks that have exceeded their TTL
func (n *Network) cleanupExpiredRecentBlocks() int {
	n.recentBlocksMu.Lock()
	defer n.recentBlocksMu.Unlock()

	removed := 0
	now := time.Now()
	for hash, seenTime := range n.recentBlocks {
		if now.Sub(seenTime) > config.RecentBlocksTTL {
			delete(n.recentBlocks, hash)
			removed++
		}
	}
	return removed
}

// cleanupExpiredRecentTransactions removes transactions that have exceeded their TTL
func (n *Network) cleanupExpiredRecentTransactions() int {
	n.recentTransactionsMu.Lock()
	defer n.recentTransactionsMu.Unlock()

	removed := 0
	now := time.Now()
	for hash, seenTime := range n.recentTransactions {
		if now.Sub(seenTime) > config.RecentTransactionsTTL {
			delete(n.recentTransactions, hash)
			removed++
		}
	}
	return removed
}

// runPeriodicCleanup runs all cleanup tasks periodically
func (n *Network) runPeriodicCleanup() {
	ticker := time.NewTicker(config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Clean up dead peers
			deadPeers := n.cleanupDeadPeers()
			if deadPeers > 0 {
				fmt.Printf("Cleaned up %d dead peers", deadPeers)
			}

			// Clean up expired recent blocks
			expiredBlocks := n.cleanupExpiredRecentBlocks()
			if expiredBlocks > 0 {
				fmt.Printf("Cleaned up %d expired recent blocks", expiredBlocks)
			}

			// Clean up expired recent transactions
			expiredTxs := n.cleanupExpiredRecentTransactions()
			if expiredTxs > 0 {
				fmt.Printf("Cleaned up %d expired recent transactions", expiredTxs)
			}

		case <-n.shutdown:
			fmt.Printf("Stopping periodic cleanup")
			return
		}
	}
}

// StartPeriodicCleanup starts the periodic cleanup tasks in a goroutine
func (n *Network) StartPeriodicCleanup() {
	go n.runPeriodicCleanup()
	fmt.Printf("Started periodic cleanup tasks (interval: %v)", config.CleanupInterval)
}

// periodicDiscovery performs periodic peer discovery tasks
func (n *Network) periodicDiscovery() {
	ticker := time.NewTicker(30 * time.Second) // Run discovery every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Run a discovery round
			<-n.RunDiscoveryRound()

		case <-n.shutdown:
			fmt.Printf("Stopping periodic discovery")
			return
		}
	}
}

// StartPeriodicDiscovery starts periodic peer discovery in a goroutine
func (n *Network) StartPeriodicDiscovery() {
	go n.periodicDiscovery()
	fmt.Printf("Started periodic peer discovery")
}
