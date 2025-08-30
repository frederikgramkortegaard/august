package main

import (
	"gocuria/blockchain"
	"gocuria/testing"
)

func main() {

	chain := blockchain.ValidateAndBuildChain(
		testing.GeneratePrebuiltChain(50, 5, 20),
	)

	_ = chain

}
