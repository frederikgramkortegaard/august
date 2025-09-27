package networking

import (
	"august/config"
	"fmt"
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
	ID         string
	Address    string // Listen address (where peer accepts connections)
	ConnAddr   string // Connection address (actual TCP endpoint, might be ephemeral)
	LastSeen   time.Time
	Status     PeerStatus
	IsOutgoing bool // True if we initiated the connection
}

// Peer management methods (integrated directly into Server)

// AddPeer adds a peer to the server's peer list
func (s *Server) AddPeer(address string) *Peer {
	s.peersMu.Lock()
	defer s.peersMu.Unlock()

	if len(s.peers) >= config.MaxPeers {
		return nil
	}
	existingPeer, exists := s.peers[address]
	if exists {
		// Peer already exists, return it so connection can proceed
		return existingPeer
	}

	s.peers[address] = &Peer{
		ID:       fmt.Sprintf("peer-%d", time.Now().Unix()),
		Address:  address,
		LastSeen: time.Now(),
		Status:   PeerConnecting,
	}

	return s.peers[address]
}

// GetConnectedPeers returns all currently connected peers
func (s *Server) GetConnectedPeers() []*Peer {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	connectedPeers := make([]*Peer, 0, config.MaxPeers)
	for _, p := range s.peers {
		if p.Status == PeerConnected {
			connectedPeers = append(connectedPeers, p)
		}
	}

	return connectedPeers
}

// CleanupDeadPeers removes peers that have been disconnected or failed for too long
func (s *Server) CleanupDeadPeers() int {
	s.peersMu.Lock()
	defer s.peersMu.Unlock()

	removed := 0
	cutoffTime := time.Now().Add(-5 * time.Minute)

	for addr, peer := range s.peers {
		// Remove peers that have been disconnected/failed for > 5 minutes
		if (peer.Status == PeerDisconnected || peer.Status == PeerFailed) &&
			peer.LastSeen.Before(cutoffTime) {
			delete(s.peers, addr)
			removed++
		}
	}

	return removed
}

// AddDiscoveredPeers adds newly discovered peer addresses to the discovered list
// Returns the count of newly added unique peers
func (s *Server) AddDiscoveredPeers(addresses []string) int {
	s.peersMu.Lock()
	defer s.peersMu.Unlock()

	// Deduplicate against existing discovered peers and current peers
	seen := make(map[string]bool)
	for _, addr := range s.discoveredPeers {
		seen[addr] = true
	}
	for addr := range s.peers {
		seen[addr] = true
	}

	newPeers := 0
	for _, addr := range addresses {
		if !seen[addr] {
			s.discoveredPeers = append(s.discoveredPeers, addr)
			seen[addr] = true
			newPeers++
		}
	}

	return newPeers
}

// GetDiscoveredPeers returns a copy of discovered peer addresses
func (s *Server) GetDiscoveredPeers() []string {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	peers := make([]string, len(s.discoveredPeers))
	copy(peers, s.discoveredPeers)
	return peers
}
