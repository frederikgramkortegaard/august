package main

import (
	"august/config"
	"august/networking"
	"august/node"
	"august/tests"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestBasicHeaderProcessing(t *testing.T) {
	_NUM_BLOCKS_TO_ADD := 3
	_NUM_NODES := 10

	_BASE_PORT := 8880
	_BASE_QUERY_PORT := 9990

	// Create test blocks chain
	testBlocks := tests.CreateBlockChainWithCoinbase(_NUM_BLOCKS_TO_ADD, config.FirstUser)

	var nodes []*node.Node
	var wg sync.WaitGroup
	for i := range _NUM_NODES {
		// Create seed peers list - Node-0 has no seeds, others connect to Node-0
		var seedPeers []string
		if i > 0 {
			seedPeers = []string{fmt.Sprintf("localhost:%d", _BASE_PORT)}
		}

		nodes = append(nodes, node.NewNode(node.NodeConfig{
			Port:      strconv.Itoa(_BASE_PORT + i),
			NodeID:    "Node-" + strconv.Itoa(i),
			SeedPeers: seedPeers,
			QueryPort: _BASE_QUERY_PORT + i,
		}))

		wg.Add(1)
		go func(n *node.Node) {
			<-n.Start()
			wg.Done()
		}(nodes[i])
	}

	wg.Wait()

	// Add longer chain to node A
	for i, block := range testBlocks[:len(testBlocks)-1] {
		err := nodes[0].AddBlockDirectly(block)
		if err != nil {
			t.Fatalf("Failed to add block %d to node A: %v", i+1, err)
		} else {
			t.Logf("Added block %s\n", networking.Hash32ToString(block.GetHash()))
		}
	}

	time.Sleep(5 * time.Second)
	// Submit block via HTTP API (like a real miner would)
	newBlock := testBlocks[len(testBlocks)-1]
	blockJSON, err := json.Marshal(newBlock)
	if err != nil {
		t.Fatalf("Failed to marshal block: %v", err)
	}
	time.Sleep(5 * time.Second)

	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/submit-block", nodes[0].Config.QueryPort), "application/json", bytes.NewBuffer(blockJSON))
	if err != nil {
		t.Fatalf("Failed to submit block via API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Log(resp)
		t.Fatalf("Block submission failed with status: %d", resp.StatusCode)

	}

	t.Logf("Block successfully submitted via API")

	// Wait for block propagation and reorganization
	t.Logf("Waiting for block propagation and reorganization...")
	time.Sleep(10 * time.Second)

	for j := range len(nodes) - 2 {
		n := nodes[j+1]
		if n.GetCurrentChain().Header.Height != uint64(_NUM_BLOCKS_TO_ADD) {
			t.Fatal("Node does not have enough blocks")
		}
		for _, block := range testBlocks {
			b := n.GetBlock(block.GetHash())
			if b == nil {
				t.Fatalf("could not find block with hash %s\n", networking.Hash32ToString(block.GetHash()))
			}
		}
	}

	// Cleanup: Stop all nodes
	for i, node := range nodes {
		t.Logf("Stopping Node-%d", i)
		node.Stop()
	}
}
