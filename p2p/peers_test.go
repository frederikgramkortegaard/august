package p2p

import (
	"testing"
	"time"
)

func TestNewPeerManager(t *testing.T) {
	seeds := []string{"127.0.0.1:9001", "127.0.0.1:9002"}
	pm := NewPeerManager(seeds)
	
	if pm == nil {
		t.Fatal("NewPeerManager returned nil")
	}
	
	if pm.maxPeers != 8 {
		t.Errorf("Expected maxPeers=8, got %d", pm.maxPeers)
	}
	
	if len(pm.seedPeers) != 2 {
		t.Errorf("Expected 2 seed peers, got %d", len(pm.seedPeers))
	}
	
	if len(pm.peers) != 0 {
		t.Errorf("Expected 0 initial peers, got %d", len(pm.peers))
	}
}

func TestAddPeer(t *testing.T) {
	pm := NewPeerManager([]string{})
	
	// Add first peer
	peer1 := pm.AddPeer("127.0.0.1:9001")
	if peer1 == nil {
		t.Fatal("Failed to add first peer")
	}
	
	if peer1.Address != "127.0.0.1:9001" {
		t.Errorf("Expected address 127.0.0.1:9001, got %s", peer1.Address)
	}
	
	if peer1.Status != PeerConnecting {
		t.Errorf("Expected status PeerConnecting, got %v", peer1.Status)
	}
	
	if len(pm.peers) != 1 {
		t.Errorf("Expected 1 peer, got %d", len(pm.peers))
	}
}

func TestAddPeerMaxLimit(t *testing.T) {
	pm := NewPeerManager([]string{})
	pm.maxPeers = 2 // Set low limit for testing
	
	// Add peers up to limit
	peer1 := pm.AddPeer("127.0.0.1:9001")
	peer2 := pm.AddPeer("127.0.0.1:9002")
	
	if peer1 == nil || peer2 == nil {
		t.Fatal("Failed to add peers within limit")
	}
	
	// Try to add beyond limit
	peer3 := pm.AddPeer("127.0.0.1:9003")
	if peer3 != nil {
		t.Error("Should not be able to add peer beyond maxPeers limit")
	}
	
	if len(pm.peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(pm.peers))
	}
}

func TestGetConnectedPeers(t *testing.T) {
	pm := NewPeerManager([]string{})
	
	// Add peers with different statuses
	peer1 := pm.AddPeer("127.0.0.1:9001")
	peer2 := pm.AddPeer("127.0.0.1:9002")
	peer3 := pm.AddPeer("127.0.0.1:9003")
	
	// Set different statuses
	peer1.Status = PeerConnected
	peer2.Status = PeerConnecting
	peer3.Status = PeerConnected
	
	connected := pm.GetConnectedPeers()
	
	if len(connected) != 2 {
		t.Errorf("Expected 2 connected peers, got %d", len(connected))
	}
	
	// Verify we got the right peers
	addresses := make(map[string]bool)
	for _, peer := range connected {
		addresses[peer.Address] = true
	}
	
	if !addresses["127.0.0.1:9001"] || !addresses["127.0.0.1:9003"] {
		t.Error("Got wrong connected peers")
	}
}

func TestPeerStatus(t *testing.T) {
	pm := NewPeerManager([]string{})
	peer := pm.AddPeer("127.0.0.1:9001")
	
	// Test initial status
	if peer.Status != PeerConnecting {
		t.Errorf("Expected initial status PeerConnecting, got %v", peer.Status)
	}
	
	// Test status changes
	peer.Status = PeerConnected
	if peer.Status != PeerConnected {
		t.Errorf("Expected status PeerConnected, got %v", peer.Status)
	}
	
	peer.Status = PeerFailed
	if peer.Status != PeerFailed {
		t.Errorf("Expected status PeerFailed, got %v", peer.Status)
	}
	
	peer.Status = PeerDisconnected
	if peer.Status != PeerDisconnected {
		t.Errorf("Expected status PeerDisconnected, got %v", peer.Status)
	}
}

func TestPeerLastSeen(t *testing.T) {
	pm := NewPeerManager([]string{})
	beforeAdd := time.Now()
	peer := pm.AddPeer("127.0.0.1:9001")
	afterAdd := time.Now()
	
	if peer.LastSeen.Before(beforeAdd) || peer.LastSeen.After(afterAdd) {
		t.Error("Peer LastSeen should be set to current time when added")
	}
}