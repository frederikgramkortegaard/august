package main

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"gocuria/node"
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
	pub, priv := GenerateKeyPair()

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

func main() {
	fmt.Println("vim-go")

	botA, err := NewBot("19001", "A", []string{})
	if err != nil {
		fmt.Println(err)
		return
	}

	botA.Node.Start()
	defer botA.Node.Stop()
}
