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
	port := flag.String("port", "9372", "Network port")
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
		*nodeID = fmt.Sprintf("node-%s-%d", *port, time.Now().Unix()%10000)
	}

	// Create node configuration
	config := node.Config{
		Port:   *port,
		NodeID:    *nodeID,
		SeedPeers: seedPeers,
	}

	// Create and start full node
	fullNode := node.NewFullNode(config)

	log.Printf("%s\tStarting full node: Network on :%s", *nodeID, *port)
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
