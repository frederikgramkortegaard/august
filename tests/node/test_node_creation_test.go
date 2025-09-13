package tests

import (
	"august/node"
	"testing"
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

	// Create and start full node
	fullNode := node.NewFullNode(testNodeConfig)

	if fullNode == nil {
		t.Errorf("Failed to initialize full node")
	}

	// Start the node and wait for it to be ready
	ready := fullNode.Start()
	<-ready // Wait for node to be ready
	fullNode.Stop()

}
