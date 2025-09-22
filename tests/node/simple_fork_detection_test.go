package main

import (
	"august/node"
	"strconv"
	"testing"
)

func TestHelloWorld(t *testing.T) {
	// t.Fatal("not implemented")

	_BASE_P2P_PORT := 8880
	_BASE_QUERY_PORT := 9990

	// Setup Initial Nodes

	n1 := node.NewNode(node.NodeConfig{
		Port:      strconv.Itoa(_BASE_P2P_PORT),
		NodeID:    "Node-0",
		SeedPeers: make([]string, 0),
		QueryPort: strconv.Itoa(_BASE_QUERY_PORT),
	})

	<-n1.Start()
}
