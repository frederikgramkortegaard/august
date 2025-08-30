package main

import (
	"fmt"
	"gocuria/blockchain"
	"gocuria/blockchain/store"
	"gocuria/testing"
	"log"
)

func main() {

	store := store.NewMemoryChainStore()

	if err := store.AddBlock(blockchain.GenesisBlock); err != nil {
		log.Fatal("Failed to add genesis:", err)
	}

	chain, _ := store.GetChain()
	fmt.Println(len(chain.Blocks))

	blocks := testing.GeneratePrebuiltChain(5, 3, 10)
	for i, block := range blocks[1:] {
		if err := store.AddBlock(block); err != nil {
			log.Printf("Failed to add block %d: %v:", i+1, err)
			break
		}
		fmt.Printf("Added block %d\n", i+1)

	}

	hash := blockchain.HashBlockHeader(&chain.Blocks[0].Header)
	fmt.Println(hash)
	fmt.Println("---")

	block, err := store.GetBlockByHash(hash)
	if err != nil {
		log.Fatal("err", err)
	}

	fmt.Println(blockchain.HashBlockHeader(&block.Header))
	fmt.Println(block == chain.Blocks[0])

}
