package contracts

import (
	"august/blockchain"
	"august/config"
	_ "august/execution"
	"august/marigold"
	"august/node"
	"august/types"
	"crypto/ed25519"
	"fmt"
	"testing"
	"time"
)

func TestPersistentStorage(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	deployerPubKey := blockchain.PublicKey(pub)

	queryPort := 18082
	port := 18083
	nodeConfig := node.NodeConfig{
		QueryPort: queryPort,
		Port:      port,
	}
	n := node.NewNode(nodeConfig)
	go n.Start()
	defer n.Stop()

	time.Sleep(500 * time.Millisecond)

	nodeAddr := fmt.Sprintf("localhost:%d", queryPort)

	t.Log("Mining initial block to fund deployer account")
	block1 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{})
	submitBlock(t, nodeAddr, block1)
	time.Sleep(200 * time.Millisecond)

	balance := getBalance(t, nodeAddr, deployerPubKey)
	if balance == 0 {
		t.Fatalf("Deployer has no balance after mining")
	}
	t.Logf("Deployer balance: %d", balance)

	// Contract that stores and retrieves values in persistent storage
	code := `
	define init() : int {
		persistent["counter"] = "0"
		persistent["owner"] = "deployer"
		emit(42)
		return 1
	}

	define call() : int {
		// Increment counter
		current = persistent["counter"]
		newValue = int(current) + 1
		persistent["counter"] = string(newValue)

		// Emit the new counter value
		emit(newValue)
		return newValue
	}
	`

	tokens, err := marigold.Lex(code)
	if err != nil {
		t.Fatalf("Failed to lex contract: %v", err)
	}

	ast, err := marigold.Parse(tokens)
	if err != nil {
		t.Fatalf("Failed to parse contract: %v", err)
	}

	ctx := marigold.CodegenWithContext(ast)
	t.Logf("Generated %d instructions", len(ctx.Instructions))

	var bytecodeStr string
	for i, inst := range ctx.Instructions {
		if inst.Opcode == types.PUSH && inst.Value != nil {
			bytecodeStr += "PUSH " + *inst.Value + " "
		} else {
			bytecodeStr += inst.Opcode.String() + " "
		}
		if (i+1)%10 == 0 {
			bytecodeStr += "\n"
		}
	}
	t.Logf("Bytecode:\n%s", bytecodeStr)

	instructions := make([]blockchain.Instruction, len(ctx.Instructions))
	for i, instr := range ctx.Instructions {
		instructions[i] = blockchain.Instruction{
			Opcode: instr.Opcode,
			Value:  instr.Value,
		}
	}

	state := getAccountState(t, nodeAddr, deployerPubKey)

	deployTx := blockchain.Transaction{
		From:         deployerPubKey,
		To:           blockchain.PublicKey{},
		Amount:       0,
		Nonce:        state.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: instructions,
		GasLimit:     100000,
		GasPrice:     100,
		Data:         nil,
	}
	deployTx.Signature = deployTx.GetSignature(priv)

	t.Log("Mining block with contract deployment")
	block2 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&deployTx})
	submitBlock(t, nodeAddr, block2)
	time.Sleep(200 * time.Millisecond)

	time.Sleep(300 * time.Millisecond)

	var contractAddr blockchain.PublicKey
	var contractState *blockchain.AccountState

	allStates, _ := getChainState(t, nodeAddr)
	t.Logf("Total accounts in chain: %d", len(allStates))
	for addr, st := range allStates {
		if len(st.Instructions) > 0 {
			contractAddr = addr
			contractState = st
			t.Logf("Found contract at %x with %d instructions", addr[:8], len(st.Instructions))
			break
		}
	}

	if contractState == nil {
		t.Fatalf("No contract found with instructions")
	}
	t.Logf("Contract has %d instructions", len(contractState.Instructions))

	// Check persistent storage after deployment
	t.Logf("Contract persistent storage after deployment:")
	for k, v := range contractState.Persistent {
		t.Logf("  %s = %s", k, v)
	}

	// Call the contract to increment counter
	t.Log("Calling contract to increment counter")
	updatedDeployerState := getAccountState(t, nodeAddr, deployerPubKey)
	callTx := blockchain.Transaction{
		From:         deployerPubKey,
		To:           contractAddr,
		Amount:       0,
		Nonce:        updatedDeployerState.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: nil,
		GasLimit:     50000,
		GasPrice:     100,
		Data:         nil,
	}
	callTx.Signature = callTx.GetSignature(priv)

	t.Log("Mining block with contract call")
	block3 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&callTx})
	submitBlock(t, nodeAddr, block3)
	time.Sleep(200 * time.Millisecond)

	// Check persistent storage after call
	finalContractState := getAccountState(t, nodeAddr, contractAddr)
	t.Logf("Contract persistent storage after call:")
	for k, v := range finalContractState.Persistent {
		t.Logf("  %s = %s", k, v)
	}

	// Verify the counter was incremented
	if finalContractState.Persistent["counter"] != "1" {
		t.Fatalf("Expected counter to be '1', got '%s'", finalContractState.Persistent["counter"])
	}

	if finalContractState.Persistent["owner"] != "deployer" {
		t.Fatalf("Expected owner to be 'deployer', got '%s'", finalContractState.Persistent["owner"])
	}

	t.Logf("Test passed: Persistent storage working correctly")
}
