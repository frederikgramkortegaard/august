package tests

import (
	"august/blockchain"
	"august/node"
	"testing"
	"time"
)

var testNodeConfig = node.Config{
	Port:      "9372",
	NodeID:    "test-node-id",
	SeedPeers: []string{""},
	DBName:    "testdb",
}

func TestNewFullNode(t *testing.T) {

	// Create and start full node
	fullNode := node.NewFullNode(testNodeConfig)
	defer fullNode.Stop()

	if fullNode == nil {
		t.Errorf("Failed to initialize full node")
	}
}

func TestStartFullNode(t *testing.T) {

	var _DESIRED_TIMEOUT_SECONDS time.Duration = 10

	// Create and start full node
	fullNode := node.NewFullNode(testNodeConfig)
	defer fullNode.Stop()

	if fullNode == nil {
		t.Errorf("Failed to initialize full node")
	}

	// Start the node and wait for it to be ready
	ready := fullNode.Start()
	select {
	case <-ready:
		// Wait for node to be ready
	case <-time.After(_DESIRED_TIMEOUT_SECONDS * time.Second):
		t.Fatal("Node failed to start within timeout")
	}

	// Ensure that the chain is accesible and loaded
	chain, err := fullNode.Store.GetChain()
	if err != nil {
		t.Errorf("Failed to fetch chain from store")
	}
	firstBlock := chain.Blocks[0]
	firstHeader := firstBlock.Header
	firstHeaderHash := blockchain.HashBlockHeader(&firstHeader)

	// Validate the Genesis block is accessible and correct
	if firstHeaderHash != blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header) {
		t.Fatal("Invalid Genesis Block Hash")
	}

	t.Log("Genesis Block Hash", firstHeaderHash)
}
