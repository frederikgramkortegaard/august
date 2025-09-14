package main

import (
	"august/node"
	"flag"
	"fmt"
	"log"
	"time"
)

func main() {
	// Command line flags for seed node
	port := flag.String("port", "9372", "Network port")
	nodeID := flag.String("id", "", "Node ID (auto-generated if not provided)")
	dbname := flag.String("dbname", "seeddb", "Name of the Pebble DB")
	minerport := flag.String("minerport", "8080", "HTTP port for miners")
	flag.Parse()

	// Auto-generate seed node ID if not provided
	if *nodeID == "" {
		*nodeID = fmt.Sprintf("seed-%s-%d", *port, time.Now().Unix()%10000)
	}

	// Create seed node configuration (no seed peers - this IS the seed!)
	config := node.Config{
		Port:      *port,
		NodeID:    *nodeID,
		SeedPeers: []string{}, // Seeds don't connect to other seeds
		DBName:    *dbname,
		QueryPort: *minerport,
	}

	// Create and start seed node
	seedNode := node.NewFullNode(config)

	log.Printf("%s\tStarting SEED node: Network on :%s", *nodeID, *port)
	log.Printf("%s\tOther nodes should connect to this address", *nodeID)

	// Start the node and wait for it to be ready
	ready := seedNode.Start()
	<-ready // Wait for node to be ready

	log.Printf("%s\tSeed node is ready and accepting connections", *nodeID)
	log.Printf("%s\tMiners can connect to HTTP API: localhost:%s", *nodeID, *minerport)

	// Keep the seed node running forever
	select {} // Block forever
}