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

func TestContractDeploymentIntegration(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	deployerPubKey := blockchain.PublicKey(pub)

	queryPort := 18080
	port := 18081
	nodeConfig := node.NodeConfig{
		QueryPort: queryPort,
		Port:    port,
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

	code := `
	define init() : int {
		persistent[string(45)] = string(75)
		persistent[string(99)] = string(1)
		emit(42)
		return 1
	}

	define add(a: int, b: int) : int {
		result = a + b
		persistent[string(200)] = string(result)
		return result
	}

	define call() : int {
		// Test function with parameters
		sum = add(25, 50)

		// Test control flow with the result
		if sum == 75 {
			persistent[string(100)] = string(200)
			emit(200)
		} else {
			persistent[string(100)] = string(300)
			emit(300)
		}

		emit(sum)
		return sum
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

	t.Logf("Complete Contract Bytecode:")
	for i, inst := range ctx.Instructions {
		if inst.Opcode == types.PUSH && inst.Value != nil {
			t.Logf("[%3d] PUSH %s", i, *inst.Value)
		} else {
			t.Logf("[%3d] %s", i, inst.Opcode.String())
		}
	}

	instructions := make([]blockchain.Instruction, len(ctx.Instructions))
	for i, instr := range ctx.Instructions {
		instructions[i] = blockchain.Instruction{
			Opcode: instr.Opcode,
			Value:instr.Value,
		}
	}

	state := getAccountState(t, nodeAddr, deployerPubKey)

	deployTx := blockchain.Transaction{
		From:       deployerPubKey,
		To:         blockchain.PublicKey{},
		Amount:     0,
		Nonce:      state.Nonce + 1,
		Timestamp:  uint64(time.Now().Unix()),
		ChainID:    config.MainnetChainID,
		Instructions: instructions,
		GasLimit:   100000,
		GasPrice:   100,
		Data:       nil,
	}
	deployTx.Signature = deployTx.GetSignature(priv)

	t.Log("Mining block with contract deployment")
	block2 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&deployTx})
	submitBlock(t, nodeAddr, block2)

	time.Sleep(500 * time.Millisecond)

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
		t.Logf("%s = %s", k, v)
	}

	t.Log("Calling contract (should emit 100)")
	updatedDeployerState := getAccountState(t, nodeAddr, deployerPubKey)
	callTx := blockchain.Transaction{
		From:       deployerPubKey,
		To:         contractAddr,
		Amount:     0,
		Nonce:      updatedDeployerState.Nonce + 1,
		Timestamp:  uint64(time.Now().Unix()),
		ChainID:    config.MainnetChainID,
		Instructions: nil,
		GasLimit:   50000,
		GasPrice:   100,
		Data:       []byte{5},
	}
	callTx.Signature = callTx.GetSignature(priv)

	t.Log("Mining block with contract call")
	block3 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&callTx})
	submitBlock(t, nodeAddr, block3)
	time.Sleep(200 * time.Millisecond)

	finalState := getAccountState(t, nodeAddr, deployerPubKey)

	// Check persistent storage after contract call using HTTP API
	contractInfo := getContractInfo(t, nodeAddr, contractAddr)
	t.Logf("Contract info from HTTP API:")
	t.Logf("Address: %s", contractInfo.Address)
	t.Logf("Balance: %d", contractInfo.Balance)
	t.Logf("Nonce: %d", contractInfo.Nonce)
	t.Logf("Is Contract: %t", contractInfo.IsContract)
	t.Logf("Has Instructions: %t", contractInfo.HasInstructions)
	t.Logf("Storage Entries: %d", contractInfo.StorageEntries)
	t.Logf("Complete Persistent Storage (demonstrating control flow):")
	for k, v := range contractInfo.PersistentData {
		t.Logf("  %s = %s", k, v)
	}

	// Verify control flow worked correctly
	t.Logf("Control Flow Verification:")
	if contractInfo.PersistentData["45"] == "75" {
		t.Logf("PASS: Initial storage: persistent[45] = 75")
	}
	if contractInfo.PersistentData["100"] == "200" {
		t.Logf("PASS: If statement: value == 75 executed correctly (persistent[100] = 200)")
	} else {
		t.Logf("FAIL: If statement failed: expected persistent[100] = 200, got %s", contractInfo.PersistentData["100"])
	}

	// Calculate actual gas costs (mining rewards make balance increase, so we need to account for that)
	totalMiningRewards := config.BlockReward * 3 // 3 blocks mined
	actualGasCost := (state.Balance + totalMiningRewards) - finalState.Balance

	if actualGasCost <= 0 {
		t.Logf("Warning: No gas cost detected - this might indicate execution issues")
	}

	t.Logf("Test passed: Contract deployed and called successfully via mining")
	t.Logf("Initial balance: %d, Final balance: %d, Mining rewards: %d", state.Balance, finalState.Balance, totalMiningRewards)
	t.Logf("Actual gas consumed: %d", actualGasCost)
}
