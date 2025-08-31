package main

import (
	"flag"
	"gocuria/node"
	"log"
	"strings"
)

func main() {
	// Command line flags
	httpPort := flag.String("http", "8372", "HTTP API port")
	p2pPort := flag.String("p2p", "9372", "P2P port")
	nodeID := flag.String("id", "", "Node ID (auto-generated if not provided)")
	seeds := flag.String("seeds", "", "Comma-separated seed peers")
	flag.Parse()

	// Parse seed peers
	var seedPeers []string
	if *seeds != "" {
		seedPeers = strings.Split(*seeds, ",")
	}

	// Create node configuration
	config := node.Config{
		HTTPPort:  *httpPort,
		P2PPort:   *p2pPort,
		NodeID:    *nodeID,
		SeedPeers: seedPeers,
	}

	// Create and start full node
	fullNode := node.NewFullNode(config)

	log.Printf("Starting full node: HTTP on :%s, P2P on :%s", *httpPort, *p2pPort)
	if len(seedPeers) > 0 {
		log.Printf("Seed peers: %v", seedPeers)
	}

	// This blocks forever
	if err := fullNode.Start(); err != nil {
		log.Fatal("Failed to start node:", err)
	}
}
