package main

import (
	_ "august/execution" // Import for side effects (registers executor)
	"august/node"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"
)

func main() {
	// Command line flags for seed node
	port := flag.Int("port", 9372, "Network port")
	nodeID := flag.String("id", "", "Node ID (auto-generated if not provided)")
	minerport := flag.String("minerport", "8080", "HTTP port for miners")
	seeds := flag.String("seeds", "", "Comma-separated list of seed nodes (e.g., localhost:9371,192.168.1.100:9372)")
	flag.Parse()

	// Auto-generate seed node ID if not provided
	if *nodeID == "" {
		*nodeID = fmt.Sprintf("seed-%s-%d", *port, time.Now().Unix()%10000)
	}

	// Parse seed peers if provided
	var seedPeers []string
	if *seeds != "" {
		// Split comma-separated seed list
		for _, seed := range strings.Split(*seeds, ",") {
			seed = strings.TrimSpace(seed)
			if seed != "" {
				seedPeers = append(seedPeers, seed)
			}
		}
	}

	// Create seed node configuration
	config := node.NodeConfig{
		Port:      *port,
		NodeID:    *nodeID,
		SeedPeers: seedPeers,
		QueryPort: *minerport,
	}

	// Create and start seed node
	seedNode := node.NewNode(config)

	log.Printf("%s\tStarting SEED node: Network on :%s", *nodeID, *port)
	if len(seedPeers) > 0 {
		log.Printf("%s\tConnecting to %d seed peers: %v", *nodeID, len(seedPeers), seedPeers)
	} else {
		log.Printf("%s\tStarting as primary seed (no peers)", *nodeID)
	}
	log.Printf("%s\tOther nodes should connect to localhost:%d", *nodeID, *port)

	// Start the node and wait for it to be ready
	ready := seedNode.Start()
	<-ready // Wait for node to be ready

	log.Printf("%s\tSeed node is ready and accepting connections", *nodeID)
	log.Printf("%s\tMiners can connect to HTTP API: localhost:%s", *nodeID, *minerport)

	// Keep the seed node running forever
	select {} // Block forever
}
