package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"gocuria/blockchain"
	"gocuria/mocks"
	"gocuria/node"
	"gocuria/p2p"
	"math/big"
	"sync"
	"time"
)

type BotConfig struct {
	pubKey  ed25519.PublicKey
	privKey ed25519.PrivateKey
}

type Bot struct {
	Config BotConfig
	Node   node.FullNode
}

func NewBot(port string, nodeid string, seeds []string) (*Bot, error) {

	if port == "" || nodeid == "" {
		return nil, errors.New("neither 'port' nor 'nodeid' can not be empty")
	}

	// PGP
	pub, priv := mocks.GenerateKeyPair()

	// Configuration
	// // Bot Config
	config := BotConfig{
		pubKey:  pub,
		privKey: priv,
	}

	// // Node Config
	nodeConfig := node.Config{
		P2PPort:   port,
		NodeID:    nodeid,
		SeedPeers: seeds,
	}

	fullNode := node.NewFullNode(nodeConfig)

	// Initializationm
	return &Bot{
		Config: config,
		Node:   *fullNode,
	}, nil
}

func (bot *Bot) MockBotBehavior() {
	// Get current chain
	fmt.Println("mocking behavior")
	server := bot.Node.GetP2PServer()
	chain, err := server.GetChainStore().GetChain()
	if err != nil {
		fmt.Printf("Failed to get chain: %v\n", err)
		return
	}

	// Create some random transactions
	var transactions []blockchain.Transaction

	// Convert pubKey to blockchain format to check balance
	var botKey blockchain.PublicKey
	copy(botKey[:], bot.Config.pubKey)
	
	// Check if bot has balance for transactions
	if botState, exists := chain.AccountStates[botKey]; exists && botState.Balance > 0 {
		// Generate transactions with sequential nonces
		currentNonce := botState.Nonce
		
		for i := 0; i < 3; i++ {
			nextNonce := currentNonce + uint64(i) + 1
			
			// Create transaction with specific nonce and random amount
			tx, err := mocks.GenerateValidTransactionWithNonce(chain, bot.Config.pubKey, bot.Config.privKey, bot.Config.pubKey, -1, nextNonce)
			if err != nil {
				fmt.Printf("Failed to generate transaction %d: %v\n", i, err)
				break // Stop if we can't generate transactions
			}
			
			fmt.Printf("Generated transaction %d with nonce %d\n", i, tx.Nonce)
			transactions = append(transactions, *tx)
		}
	}

	// Generate a valid mined block with these transactions
	block, err := mocks.GenerateValidMinedBlock(chain, bot.Config.pubKey, transactions)
	if err != nil {
		fmt.Printf("Failed to generate mined block: %v\n", err)
		return
	}
	fmt.Printf("Bot %s mined block with %d transactions\n", bot.Node.GetNodeID(), len(transactions))

	// Add block to own chain
	if block != nil {
		<-p2p.ProcessBlock(bot.Node.P2PServer, block, "0.0.0.0:"+bot.Node.Config.P2PPort)
	}

}

func (bot *Bot) StartPeriodicBehavior() {
	go func() {
		for {
			// Generate random interval between 10 seconds and 2 minutes (120 seconds)
			minSeconds := 10
			maxSeconds := 120
			rangeSeconds := maxSeconds - minSeconds
			
			randomBig, _ := rand.Int(rand.Reader, big.NewInt(int64(rangeSeconds)))
			randomSeconds := minSeconds + int(randomBig.Int64())
			
			fmt.Printf("Bot %s will mine next block in %d seconds\n", bot.Node.GetNodeID(), randomSeconds)
			
			time.Sleep(time.Duration(randomSeconds) * time.Second)
			bot.MockBotBehavior()
		}
	}()
}

type NodeManager struct {
	bots           map[string]*Bot
	mu             sync.RWMutex
	nodeCounter    int
	activePeers    []string // List of active peer addresses for bootstrapping
}

func NewNodeManager(initialPeers []string) *NodeManager {
	return &NodeManager{
		bots:        make(map[string]*Bot),
		activePeers: append([]string(nil), initialPeers...), // Copy slice
	}
}

func (nm *NodeManager) AddBot(nodeID string, bot *Bot) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.bots[nodeID] = bot
	
	// Add this node's address to active peers list
	peerAddr := "0.0.0.0:" + bot.Node.Config.P2PPort
	nm.activePeers = append(nm.activePeers, peerAddr)
}

func (nm *NodeManager) RemoveBot(nodeID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	if bot, exists := nm.bots[nodeID]; exists {
		// Remove from active peers list
		peerAddr := "0.0.0.0:" + bot.Node.Config.P2PPort
		for i, addr := range nm.activePeers {
			if addr == peerAddr {
				nm.activePeers = append(nm.activePeers[:i], nm.activePeers[i+1:]...)
				break
			}
		}
		
		bot.Node.Stop()
		delete(nm.bots, nodeID)
		fmt.Printf("NETWORK: Removed node %s from network\n", nodeID)
	}
}

func (nm *NodeManager) GetRandomBotID() string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	if len(nm.bots) <= 2 { // Keep at least 2 nodes for network viability
		return ""
	}
	
	// Get all bot IDs - any node can be removed now
	var candidates []string
	for nodeID := range nm.bots {
		candidates = append(candidates, nodeID)
	}
	
	if len(candidates) == 0 {
		return ""
	}
	
	// Pick random candidate
	randomBig, _ := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
	return candidates[randomBig.Int64()]
}

func (nm *NodeManager) CreateFreshBot() {
	nm.mu.Lock()
	nm.nodeCounter++
	nodeID := fmt.Sprintf("dynamic-bot-%d", nm.nodeCounter)
	port := fmt.Sprintf("1910%d", nm.nodeCounter%10) // Use ports 19100-19109
	
	// Get current active peers for bootstrapping (copy to avoid race)
	var bootstrapPeers []string
	if len(nm.activePeers) > 0 {
		// Use up to 3 random active peers as bootstrap nodes
		maxPeers := 3
		if len(nm.activePeers) < maxPeers {
			maxPeers = len(nm.activePeers)
		}
		
		// Shuffle and take first maxPeers
		peersCopy := append([]string(nil), nm.activePeers...)
		for i := len(peersCopy) - 1; i > 0; i-- {
			randomBig, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
			j := int(randomBig.Int64())
			peersCopy[i], peersCopy[j] = peersCopy[j], peersCopy[i]
		}
		bootstrapPeers = peersCopy[:maxPeers]
	}
	nm.mu.Unlock()
	
	if len(bootstrapPeers) == 0 {
		fmt.Printf("NETWORK: No active peers available for fresh node %s\n", nodeID)
		return
	}
	
	bot, err := NewBot(port, nodeID, bootstrapPeers)
	if err != nil {
		fmt.Printf("NETWORK: Failed to create fresh node %s: %v\n", nodeID, err)
		return
	}
	
	<-bot.Node.Start() // Wait for node to be ready
	bot.StartPeriodicBehavior()
	
	nm.AddBot(nodeID, bot)
	fmt.Printf("NETWORK: Added fresh node %s to network on port %s (bootstrapped from %v)\n", nodeID, port, bootstrapPeers)
}

func (nm *NodeManager) StartDynamicNodeManagement() {
	go func() {
		for {
			// Wait 2 minutes
			time.Sleep(2 * time.Minute)
			
			// Remove a random node (if we have more than just seed)
			targetID := nm.GetRandomBotID()
			if targetID != "" {
				nm.RemoveBot(targetID)
			}
			
			// Wait 10 seconds before adding replacement
			time.Sleep(10 * time.Second)
			
			// Add a fresh node
			nm.CreateFreshBot()
		}
	}()
}

func main() {
	// Create node manager with no permanent seeds
	nodeManager := NewNodeManager([]string{})

	// Start initial bootstrap node
	bootstrapPort := "19000"
	bootstrapBot, err := NewBot(bootstrapPort, "bootstrap-node", []string{})
	if err != nil {
		fmt.Println(err)
		return
	}
	<-bootstrapBot.Node.Start() // Wait for bootstrap to be ready
	defer bootstrapBot.Node.Stop()
	bootstrapBot.StartPeriodicBehavior()
	nodeManager.AddBot("bootstrap-node", bootstrapBot)
	
	fmt.Printf("NETWORK: Bootstrap node started on port %s\n", bootstrapPort)

	// Start initial batch of bots that connect to bootstrap node
	numInitialBots := 6
	for i := 1; i <= numInitialBots; i++ {
		botName := fmt.Sprintf("genesis-bot-%d", i)
		port := fmt.Sprintf("1900%d", i)
		bot, err := NewBot(port, botName, []string{"0.0.0.0:" + bootstrapPort})
		if err != nil {
			fmt.Printf("Failed to create bot %s: %v\n", botName, err)
			continue
		}
		
		<-bot.Node.Start() // Wait for bot to be ready
		defer bot.Node.Stop()
		bot.StartPeriodicBehavior()
		nodeManager.AddBot(botName, bot)
		
		fmt.Printf("NETWORK: Initial bot %s started on port %s\n", botName, port)
	}

	// Start dynamic node management (ANY node can be dropped now, including bootstrap)
	nodeManager.StartDynamicNodeManagement()
	fmt.Println("NETWORK: Dynamic node management started - ANY node (including bootstrap) can join/leave every 2 minutes")
	fmt.Println("NETWORK: Testing true decentralized network with rotating peers")

	// Keep running
	select {}
}
