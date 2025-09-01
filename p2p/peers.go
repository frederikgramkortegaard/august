package p2p

import (
	"fmt"
	"sync"
	"time"
)

type PeerStatus int

const (
	PeerDisconnected PeerStatus = iota
	PeerConnecting
	PeerConnected
	PeerFailed
)

type Peer struct {
	ID       string
	Address  string
	LastSeen time.Time
	Status   PeerStatus
}

type PeerManager struct {
	peers           map[string]*Peer
	maxPeers        int
	currentPeers    int
	seedPeers       []string
	discoveredPeers []string     // Peers learned from other peers
	mu              sync.RWMutex // Protects the peers map and discoveredPeers
}

func NewPeerManager(seeds []string) *PeerManager {
	return &PeerManager{
		peers:           make(map[string]*Peer),
		maxPeers:        128,
		currentPeers:    0,
		seedPeers:       seeds,
		discoveredPeers: make([]string, 0),
	}
}

func (pm *PeerManager) AddPeer(address string) *Peer {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.currentPeers >= pm.maxPeers {
		return nil
	}
	existingPeer, exists := pm.peers[address]
	if exists {
		// Peer already exists, return it so connection can proceed
		return existingPeer
	}

	pm.peers[address] = &Peer{
		ID:       fmt.Sprintf("peer-%d", time.Now().Unix()),
		Address:  address,
		LastSeen: time.Now(),
		Status:   PeerConnecting,
	}

	pm.currentPeers += 1

	return pm.peers[address]
}

func (pm *PeerManager) GetConnectedPeers() []*Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	connectedPeers := make([]*Peer, 0, pm.maxPeers)
	for _, p := range pm.peers {
		if p.Status == PeerConnected {
			connectedPeers = append(connectedPeers, p)
		}
	}

	return connectedPeers
}

// CleanupDeadPeers removes peers that have been disconnected or failed for too long
func (pm *PeerManager) CleanupDeadPeers() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	removed := 0
	cutoffTime := time.Now().Add(-5 * time.Minute)
	
	for addr, peer := range pm.peers {
		// Remove peers that have been disconnected/failed for > 5 minutes
		if (peer.Status == PeerDisconnected || peer.Status == PeerFailed) &&
			peer.LastSeen.Before(cutoffTime) {
			delete(pm.peers, addr)
			pm.currentPeers--
			removed++
		}
	}
	
	return removed
}

// AddDiscoveredPeers adds newly discovered peer addresses to the discovered list
// Returns the count of newly added unique peers
func (pm *PeerManager) AddDiscoveredPeers(addresses []string) int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Deduplicate against existing discovered peers and current peers
	seen := make(map[string]bool)
	for _, addr := range pm.discoveredPeers {
		seen[addr] = true
	}
	for addr := range pm.peers {
		seen[addr] = true
	}
	
	newPeers := 0
	for _, addr := range addresses {
		if !seen[addr] {
			pm.discoveredPeers = append(pm.discoveredPeers, addr)
			seen[addr] = true
			newPeers++
		}
	}
	
	return newPeers
}

// GetDiscoveredPeers returns a copy of discovered peer addresses
func (pm *PeerManager) GetDiscoveredPeers() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	peers := make([]string, len(pm.discoveredPeers))
	copy(peers, pm.discoveredPeers)
	return peers
}
