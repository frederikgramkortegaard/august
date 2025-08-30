package main

import (
	"gocuria/src/blockchain"
	"gocuria/src/testing"
)

func main() {

	chain := blockchain.ValidateAndBuildChain(
		testing.GeneratePrebuiltChain(50, 5, 20),
	)

	_ = chain

}
