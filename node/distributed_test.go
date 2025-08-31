package node

import (
	"encoding/json"
	"fmt"
	"gocuria/blockchain"
	"gocuria/p2p"
	"net"
	"sync"
	"testing"
	"time"
)

// TestDistributedP2PBlockPropagation tests block propagation across multiple nodes
func TestDistributedP2PBlockPropagation(t *testing.T) {
	// Create 3 nodes that will form a network
	numNodes := 3
	nodes := make([]*FullNode, numNodes)
	nodeConfigs := make([]Config, numNodes)
	
	// Start with different ports for each node
	baseHTTPPort := 18000
	baseP2PPort := 19000
	
	// Setup node configurations
	for i := 0; i < numNodes; i++ {
		nodeConfigs[i] = Config{
			HTTPPort: fmt.Sprintf("%d", baseHTTPPort+i),
			P2PPort:  fmt.Sprintf("%d", baseP2PPort+i),
			NodeID:   fmt.Sprintf("test-node-%d", i),
		}
	}
	
	// Set up seed peers (each node connects to the previous one)
	for i := 1; i < numNodes; i++ {
		// Node i connects to all previous nodes
		var seeds []string
		for j := 0; j < i; j++ {
			seeds = append(seeds, fmt.Sprintf("localhost:%d", baseP2PPort+j))
		}
		nodeConfigs[i].SeedPeers = seeds
	}
	
	// Start all nodes
	var wg sync.WaitGroup
	for i := 0; i < numNodes; i++ {
		wg.Add(1)
		go func(nodeIndex int) {
			defer wg.Done()
			
			nodes[nodeIndex] = NewFullNode(nodeConfigs[nodeIndex])
			
			// Start the node in background
			go func() {
				if err := nodes[nodeIndex].Start(); err != nil {
					t.Errorf("Failed to start node %d: %v", nodeIndex, err)
				}
			}()
			
			t.Logf("Node %d started on HTTP:%s P2P:%s", 
				nodeIndex, nodeConfigs[nodeIndex].HTTPPort, nodeConfigs[nodeIndex].P2PPort)
		}(i)
	}
	
	// Wait for all nodes to start
	wg.Wait()
	
	// Give nodes time to discover each other
	t.Log("Waiting for nodes to discover each other...")
	time.Sleep(3 * time.Second)
	
	// Check how many peers each node has connected
	for i, node := range nodes {
		if node.p2pServer != nil {
			connectedPeers := node.p2pServer.GetPeerManager().GetConnectedPeers()
			t.Logf("Node %d has %d connected peers", i, len(connectedPeers))
		}
	}
	
	// Create a test block to propagate
	testBlock := createTestBlock(t)
	
	// Inject the block directly into node 0
	blockHash := blockchain.HashBlockHeader(&testBlock.Header)
	t.Logf("Injecting test block %x into node 0", blockHash[:8])
	if err := nodes[0].ProcessBlock(testBlock); err != nil {
		t.Fatalf("Failed to process test block in node 0: %v", err)
	}
	
	// Give time for block propagation
	t.Log("Waiting for block propagation...")
	time.Sleep(3 * time.Second)
	
	// Check that all nodes have the block
	// blockHash already defined above
	for i, node := range nodes {
		chain, err := node.store.GetChain()
		if err != nil {
			t.Errorf("Node %d: Failed to get chain: %v", i, err)
			continue
		}
		
		// Look for our test block
		found := false
		for _, block := range chain.Blocks {
			if blockchain.HashBlockHeader(&block.Header) == blockHash {
				found = true
				break
			}
		}
		
		if !found {
			t.Errorf("Node %d: Test block %x was not found in chain", i, blockHash[:8])
		} else {
			t.Logf("Node %d: Successfully has test block %x", i, blockHash[:8])
		}
		
		t.Logf("Node %d: Chain has %d blocks", i, len(chain.Blocks))
	}
	
	// Cleanup - close all nodes
	for i, node := range nodes {
		if err := node.Stop(); err != nil {
			t.Logf("Error stopping node %d: %v", i, err)
		}
		t.Logf("Node %d cleaned up", i)
	}
}

// TestOrphanBlockHandling tests the orphan block system across multiple nodes
func TestOrphanBlockHandling(t *testing.T) {
	// Start 2 nodes
	node1 := NewFullNode(Config{
		HTTPPort: "18010",
		P2PPort:  "19010",
		NodeID:   "test-node-orphan-1",
	})
	
	node2 := NewFullNode(Config{
		HTTPPort: "18011", 
		P2PPort:  "19011",
		NodeID:   "test-node-orphan-2",
		SeedPeers: []string{"localhost:19010"},
	})
	
	// Start nodes
	go node1.Start()
	go node2.Start()
	
	// Wait for connection
	time.Sleep(2 * time.Second)
	
	// Create two blocks: block A and block B (where B references A)
	blockA := createTestBlock(t)
	blockB := createTestBlockWithParent(t, blockchain.HashBlockHeader(&blockA.Header))
	
	// Send block B first (orphan) to node 2
	blockBHash := blockchain.HashBlockHeader(&blockB.Header)
	t.Logf("Sending orphan block B %x to node 2", blockBHash[:8])
	if err := node2.ProcessBlock(blockB); err != nil {
		t.Logf("Expected: Block B processing failed (orphan): %v", err)
	}
	
	// Check that block B is in orphan pool
	if len(node2.orphanPool) != 1 {
		t.Errorf("Expected 1 block in orphan pool, got %d", len(node2.orphanPool))
	}
	
	// Now send block A to node 2 (should connect the orphan)
	blockAHash := blockchain.HashBlockHeader(&blockA.Header)
	t.Logf("Sending parent block A %x to node 2", blockAHash[:8])
	if err := node2.ProcessBlock(blockA); err != nil {
		t.Fatalf("Failed to process parent block A: %v", err)
	}
	
	// Wait for orphan connection
	time.Sleep(1 * time.Second)
	
	// Check that orphan pool is now empty (block B should be connected)
	if len(node2.orphanPool) != 0 {
		t.Errorf("Expected 0 blocks in orphan pool after connection, got %d", len(node2.orphanPool))
	}
	
	// Check that both blocks are in the chain
	chain, _ := node2.store.GetChain()
	if len(chain.Blocks) < 3 { // genesis + A + B
		t.Errorf("Expected at least 3 blocks in chain, got %d", len(chain.Blocks))
	}
	
	// Cleanup
	if node1.p2pServer != nil {
		node1.p2pServer.Stop()
	}
	if node2.p2pServer != nil {
		node2.p2pServer.Stop()
	}
}

// Helper function to create a test block
func createTestBlock(t *testing.T) *blockchain.Block {
	// Create a simple coinbase transaction
	coinbase := blockchain.Transaction{
		From:   blockchain.PublicKey{}, // Zero key = coinbase
		To:     blockchain.FirstUser,
		Amount: 1000,
		Nonce:  0,
	}
	
	// Create block with the coinbase transaction
	merkle := blockchain.MerkleTransactions([]blockchain.Transaction{coinbase})
	
	header := blockchain.BlockHeader{
		Version:      1,
		PreviousHash: blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header),
		Timestamp:    uint64(time.Now().Unix()),
		Nonce:        0,
		MerkleRoot:   merkle,
	}
	
	// Mine the block (find valid nonce)
	nonce, err := blockchain.MineCorrectNonce(&header, 1)
	if err != nil {
		t.Fatalf("Failed to mine test block: %v", err)
	}
	header.Nonce = nonce
	
	return &blockchain.Block{
		Header:       header,
		Transactions: []blockchain.Transaction{coinbase},
	}
}

// Helper function to create a test block that references a specific parent
func createTestBlockWithParent(t *testing.T, parentHash blockchain.Hash32) *blockchain.Block {
	// Create a simple coinbase transaction
	coinbase := blockchain.Transaction{
		From:   blockchain.PublicKey{}, // Zero key = coinbase
		To:     blockchain.FirstUser,
		Amount: 1000,
		Nonce:  0,
	}
	
	// Create block with the coinbase transaction
	merkle := blockchain.MerkleTransactions([]blockchain.Transaction{coinbase})
	
	header := blockchain.BlockHeader{
		Version:      1,
		PreviousHash: parentHash, // Reference the specific parent
		Timestamp:    uint64(time.Now().Unix()) + 1, // Slightly later timestamp
		Nonce:        0,
		MerkleRoot:   merkle,
	}
	
	// Mine the block (find valid nonce)
	nonce, err := blockchain.MineCorrectNonce(&header, 1)
	if err != nil {
		t.Fatalf("Failed to mine test block with parent: %v", err)
	}
	header.Nonce = nonce
	
	return &blockchain.Block{
		Header:       header,
		Transactions: []blockchain.Transaction{coinbase},
	}
}

// TestP2PMessageExchange tests basic P2P message exchange
func TestP2PMessageExchange(t *testing.T) {
	// Create a simple test of P2P message sending
	node1 := NewFullNode(Config{
		HTTPPort: "18020",
		P2PPort:  "19020", 
		NodeID:   "test-node-msg-1",
	})
	
	// Start node
	go node1.Start()
	time.Sleep(1 * time.Second)
	
	// Connect directly to the P2P port and send a message
	conn, err := net.Dial("tcp", "localhost:19020")
	if err != nil {
		t.Fatalf("Failed to connect to P2P port: %v", err)
	}
	defer conn.Close()
	
	// Send a handshake message
	handshake := p2p.HandshakePayload{
		NodeID:      "test-client",
		ChainHeight: 1,
		Version:     "1.0",
	}
	
	msg, err := p2p.NewMessage(p2p.MessageTypeHandshake, handshake)
	if err != nil {
		t.Fatalf("Failed to create handshake message: %v", err)
	}
	
	// Send the message
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		t.Fatalf("Failed to send handshake: %v", err)
	}
	
	// Read response (should be a handshake back)
	decoder := json.NewDecoder(conn)
	var response p2p.Message
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Failed to read handshake response: %v", err)
	}
	
	if response.Type != p2p.MessageTypeHandshake {
		t.Errorf("Expected handshake response, got %s", response.Type)
	}
	
	t.Log("P2P message exchange successful")
	
	// Cleanup
	if node1.p2pServer != nil {
		node1.p2pServer.Stop()
	}
}