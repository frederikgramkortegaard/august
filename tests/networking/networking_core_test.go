package tests

import (
	"august/node"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

var testNodeConfig node.NodeConfig

const NUM_NODES = 10
const PORT_START_RANGE = 9370

var tempDir string // Shared temp directory for all tests

// TestMain runs before all tests and sets up temporary directories
func TestMain(m *testing.M) {
	// Create a temporary directory for all tests
	var err error
	tempDir, err = os.MkdirTemp("", "august-network-tests-*")
	if err != nil {
		panic("Failed to create temp directory: " + err.Error())
	}

	// Run all tests
	code := m.Run()

	// Cleanup: Remove temporary directory
	os.RemoveAll(tempDir)

	// Exit with the same code as the tests
	os.Exit(code)
}

// createTestNodes creates a fresh batch of nodes for testing
func createTestNodes(nodeCount int) []*node.FullNode {
	if nodeCount > NUM_NODES {
		panic("Requested more nodes than available ports")
	}

	nodes := make([]*node.FullNode, nodeCount)

	for idx := 0; idx < nodeCount; idx++ {
		portAsString := strconv.Itoa(PORT_START_RANGE + idx)

		var seedPeers []string
		if idx > 0 {
			// Client nodes seed to the first node (seed)
			seedPeers = []string{"127.0.0.1:" + strconv.Itoa(PORT_START_RANGE)}
		}

		conf := node.NodeConfig{
			Port:      portAsString,
			NodeID:    "test-node-id-" + portAsString,
			SeedPeers: seedPeers,
			DBName:    filepath.Join(tempDir, fmt.Sprintf("test-%d-db-%s", idx, portAsString)),
		}

		nodes[idx] = node.NewFullNode(conf)
		if nodes[idx] == nil {
			panic(fmt.Sprintf("Node %d was not created properly", idx))
		}
	}

	return nodes
}

// stopTestNodes gracefully stops all nodes
func stopTestNodes(nodes []*node.FullNode) {
	var wg sync.WaitGroup
	for _, n := range nodes {
		if n != nil {
			wg.Add(1)
			go func(n *node.FullNode) {
				defer wg.Done()
				n.Stop()
			}(n)
		}
	}
	wg.Wait()
}

// setupNetworkingNodes creates nodes and starts their networking
// Returns the nodes - caller is responsible for cleanup with defer stopTestNodes(nodes)
func setupNetworkingNodes(t *testing.T, nodeCount int) []*node.FullNode {
	nodes := createTestNodes(nodeCount)

	var wg sync.WaitGroup

	// Start networking for all nodes
	for idx, n := range nodes {
		wg.Add(1)
		go func(id int, node *node.FullNode) {
			defer wg.Done()
			if !<-node.StartNetworking() {
				t.Errorf("Node %d failed to start networking", id)
				return
			}
			t.Logf("Node %d networking started", id)
		}(idx, n)
	}

	wg.Wait() // Wait for all networking to start
	return nodes
}

// connectSeeds connects all non-seed nodes to the seed (first node) with random delays
func connectSeeds(t *testing.T, nodes []*node.FullNode) {
	var wg sync.WaitGroup

	for idx, n := range nodes {
		if idx > 0 { // Non-seed nodes connect to seed
			wg.Add(1)
			go func(id int, node *node.FullNode) {
				defer wg.Done()
				// Add random delay to stagger connection attempts (0-500ms)
				delay := time.Duration(1000) * time.Millisecond
				time.Sleep(delay)

				if !<-node.ConnectToSeeds() {
					t.Errorf("Node %d failed to connect to seeds", id)
				} else {
					t.Logf("Node %d connected to seeds", id)
				}
			}(idx, n)
		}
	}

	wg.Wait() // Wait for all seed connections to complete
}

// startDiscovery starts peer discovery on all nodes with random delays
func startDiscovery(t *testing.T, nodes []*node.FullNode) {
	var wg sync.WaitGroup

	for idx, n := range nodes {
		wg.Add(1)
		go func(id int, node *node.FullNode) {
			defer wg.Done()
			// Add random delay to stagger discovery attempts (0-200ms)
			delay := time.Duration(rand.Intn(200)) * time.Millisecond
			time.Sleep(delay)

			if !<-node.NetworkServer.RunDiscoveryRound() {
				t.Errorf("Node %d failed to start discovery", id)
			} else {
				t.Logf("Node %d started discovery", id)
			}
		}(idx, n)
	}

	wg.Wait() // Wait for all nodes to start discovery
}

func TestSeedConnection(t *testing.T) {
	nodes := setupNetworkingNodes(t, NUM_NODES)
	defer stopTestNodes(nodes)

	// Connect seeds
	connectSeeds(t, nodes)

	// Verify connections: non-seed nodes should have peer connections
	for idx, n := range nodes {
		peerCount := len(n.GetNetworkServer().GetConnectedPeers())
		if idx == 0 {
			// Seed should have connections from other nodes
			if peerCount != NUM_NODES-1 {
				t.Errorf("Seed node has %d peers, expected %d", peerCount, NUM_NODES-1)
			} else {
				t.Logf("Seed node has %d connected peers", peerCount)
			}
		} else {
			// Non-seed should be connected to seed
			if peerCount < 1 {
				t.Errorf("Node %d has %d peers, expected at least 1", idx, peerCount)
			} else {
				t.Logf("Node %d has %d connected peers", idx, peerCount)
			}
		}
	}
}

func TestDiscovery(t *testing.T) {

	_MAX_MISSING_PEERS := 2

	nodes := setupNetworkingNodes(t, NUM_NODES)
	defer stopTestNodes(nodes)

	// Connect seeds first
	connectSeeds(t, nodes)

	// Wait a bit for seed connections to stabilize
	time.Sleep(1000 * time.Millisecond)

	startDiscovery(t, nodes)
	time.Sleep(1000 * time.Millisecond) // Allow time for connections to complete

	startDiscovery(t, nodes)
	time.Sleep(1000 * time.Millisecond)

	startDiscovery(t, nodes)
	time.Sleep(1000 * time.Millisecond)

	for idx, n := range nodes {
		connectedPeers := n.GetNetworkServer().GetConnectedPeers()
		if (NUM_NODES-1)-len(connectedPeers) > _MAX_MISSING_PEERS {
			t.Fatalf("Node %d only had %d connections but expected %d\n", idx, len(connectedPeers), NUM_NODES-1)
		} else {
			t.Logf("Node %d has %d connected peers", idx, len(connectedPeers))
		}
	}

}
