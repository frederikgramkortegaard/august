package node

import (
	"august/blockchain"
	"august/libnet"
	"august/libnet/rpc"
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"log"
	"time"
)

// NewNode creates a node that runs all services
func NewNode(config NodeConfig) *Node {
	// Create mempool
	nodeMempool := NewMempool()

	// Create context for goroutine management
	ctx, cancel := context.WithCancel(context.Background())

	node := &Node{
		Config:          config,
		Mempool:         nodeMempool,
		chains:          make(map[blockchain.Hash32]*blockchain.Chain),
		headersMap:      make(map[blockchain.Hash32]*blockchain.BlockHeader),
		blocksMap:       make(map[blockchain.Hash32]*blockchain.Block),
		orphanHeaders:   make(map[blockchain.Hash32][]*blockchain.BlockHeader),
		submittedBlocks: make(map[blockchain.Hash32]*blockchain.Block),
		ctx:             ctx,
		cancel:          cancel,
	}

	// PeerService
	// Setup Seeds as LibP2P Multiaddr
	var seeds []peer.AddrInfo

	for _, s := range config.SeedPeers {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			log.Printf("%s\tFailed to parse seed multiaddr %s: %v", config.NodeID, s, err)
			continue
		}
		ai, err := peer.AddrInfoFromP2pAddr(m)
		if err != nil {
			log.Printf("%s\tFailed to convert seed to AddrInfo %s: %v", config.NodeID, s, err)
			continue
		}
		seeds = append(seeds, *ai)
	}

	// Initialize PeerService (Starts on Initialization)
	ps, err := libnet.NewPeerService(
		"127.0.0.1", //@TODO : Why is this not in NodeConfig
		config.Port,
		ctx,

		// Initiate the Protocols we want
		func(ps *libnet.PeerService) {
			_ = rpc.NewRPCProtocol(ps)
		},

		// Bootstrap seed list
		seeds...,
	)
	if err != nil {
		panic(err)
	}
	ps.Node = node
	node.PeerService = ps

	// Set up RPC callbacks for announcements
	// RPC is set during the initProtocols callback above, so we can use it now
	if ps.RPC != nil {
		ps.RPC.SetOnHeaderAnnouncementCallback(func(header *blockchain.BlockHeader) {
			// Handle incoming header announcements
			node.ProcessHeader(header)
		})

		ps.RPC.SetOnTransactionAnnouncementCallback(func(tx *blockchain.Transaction) {
			// Handle incoming transaction announcements
			node.ProcessTransaction(tx)
		})
	}

	// Initialize with genesis chain
	genesisChain := blockchain.NewChain()
	genesisChain.AddHeader(&blockchain.GenesisBlock.Header)
	genesisChain.AddBlock(blockchain.GenesisBlock) // Add the full genesis block
	genesisHash := blockchain.GenesisBlock.Header.GetHash()
	node.chains[genesisHash] = genesisChain
	node.currentChainTip = genesisHash

	// Add genesis header and block to global maps
	node.headersMap[genesisHash] = &blockchain.GenesisBlock.Header
	node.blocksMap[genesisHash] = blockchain.GenesisBlock

	return node
}

// Start initializes and starts all components
func (n *Node) Start() <-chan bool {
	ready := make(chan bool, 1)

	go func() {

		// Start HTTP API for miners and queries if configured
		if n.Config.QueryPort != 0 {
			go func() {
				if err := StartQueryAPI(n, n.Config.QueryPort); err != nil {
					log.Printf("%s\tQuery API server failed: %v", n.Config.NodeID, err)
				}
			}()
		}

		log.Printf("%s\tFull node started: Network on :%d", n.Config.NodeID, n.Config.Port)

		// Signal ready
		ready <- true

		// Block forever
		select {}
	}()

	return ready
}

// Stop gracefully shuts down the Node
func (n *Node) Stop() error {
	log.Printf("%s\tStopping Node...", n.Config.NodeID)

	// Signal all goroutines to stop
	if n.cancel != nil {
		n.cancel()
	}

	if n.PeerService != nil {
		n.PeerService.Close()
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("%s\tAll goroutines stopped gracefully", n.Config.NodeID)
	case <-time.After(5 * time.Second):
		log.Printf("%s\tTimeout waiting for goroutines to stop", n.Config.NodeID)
	}

	log.Println("Node stopped successfully")
	return nil
}
