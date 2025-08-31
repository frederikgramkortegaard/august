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
	peers     map[string]*Peer
	maxPeers  int
	currentPeers	int
	seedPeers []string
	mu        sync.RWMutex // Protects the peers map
}

func NewPeerManager(seeds []string) *PeerManager {
	return &PeerManager{
		peers:     make(map[string]*Peer),
		maxPeers:  128,
		currentPeers: 0,
		seedPeers: seeds,
	}
}

func (pm *PeerManager) AddPeer(address string) *Peer {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if pm.currentPeers >= pm.maxPeers {
		return nil
	}
	_, ok := pm.peers[address]
	if ok {
		return nil
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
