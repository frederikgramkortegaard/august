package main

import (
	"august/node"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"
)

func main() {
	// Command line flags
	p2pPort := flag.String("p2p", "9372", "P2P port")
	nodeID := flag.String("id", "", "Node ID (auto-generated if not provided)")
	seeds := flag.String("seeds", "", "Comma-separated seed peers")
	flag.Parse()

	// Parse seed peers
	var seedPeers []string
	if *seeds != "" {
		seedPeers = strings.Split(*seeds, ",")
	}

	// Auto-generate node ID if not provided
	if *nodeID == "" {
		*nodeID = fmt.Sprintf("node-%s-%d", *p2pPort, time.Now().Unix()%10000)
	}

	// Create node configuration
	config := node.Config{
		P2PPort:   *p2pPort,
		NodeID:    *nodeID,
		SeedPeers: seedPeers,
	}

	// Create and start full node
	fullNode := node.NewFullNode(config)

	log.Printf("%s\tStarting full node: P2P on :%s", *nodeID, *p2pPort)
	if len(seedPeers) > 0 {
		log.Printf("%s\tSeed peers: %v", *nodeID, seedPeers)
	}

	// Start the node and wait for it to be ready
	ready := fullNode.Start()
	<-ready // Wait for node to be ready

	log.Printf("%s\tNode is ready", *nodeID)

	// Keep the node running
	select {} // Block forever
}
