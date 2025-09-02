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

func main() {

	// Seed Node
	seedNodePort := "19000"
	botA, err := NewBot(seedNodePort, "seedNode", []string{})
	if err != nil {
		fmt.Println(err)
		return
	}
	botA.Node.Start()
	defer botA.Node.Stop()
	botA.StartPeriodicBehavior()

	numBots := 6
	bots := make(map[string]*Bot)
	for i := range numBots {
		botName := fmt.Sprintf("bot-%d", i+1)
		port := fmt.Sprintf("1900%d", i+1)
		bots[botName], err = NewBot(port, botName, []string{"0.0.0.0:" + seedNodePort})
		bots[botName].Node.Start()
		defer bots[botName].Node.Stop()
		bots[botName].StartPeriodicBehavior()
	}

	select {}
}
