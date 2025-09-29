package contracts

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"august/blockchain"
	"august/config"
	"august/marigold"
	"august/node"
)

func TestSimpleTransferComparison(t *testing.T) {
	// Generate keys for testing
	deployerPub, deployerPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	deployerPubKey := blockchain.PublicKey(deployerPub)

	userPub, userPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	userPubKey := blockchain.PublicKey(userPub)

	// Start node for testing
	queryPort := 18084
	port := 18085
	nodeConfig := node.NodeConfig{
		QueryPort: queryPort,
		Port:      port,
	}
	n := node.NewNode(nodeConfig)
	go n.Start()
	defer n.Stop()

	time.Sleep(500 * time.Millisecond)
	nodeAddr := fmt.Sprintf("localhost:%d", queryPort)

	// Mine initial block to fund deployer
	block1 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{})
	submitBlock(t, nodeAddr, block1)
	time.Sleep(200 * time.Millisecond)

	// Deploy the simple transfer test contract
	simpleContract := `
define init() : int {
	return 1
}

define call() : int {
	if len(@tsxdata) >= 8 {
		command: string = @tsxdata[:8]
		emit(999) // About to compare
		if command == "transfer" {
			emit(111) // Transfer match!
			return 1
		} else {
			emit(222) // No match
			return 0
		}
	}

	emit(333) // Too short
	return 0
}`

	t.Log("Compiling simple transfer test contract")

	// Parse the contract
	tokens, err := marigold.Lex(simpleContract)
	require.NoError(t, err, "Lexing should succeed")

	ast, err := marigold.Parse(tokens)
	require.NoError(t, err, "Parsing should succeed")

	// Type check
	marigold.Typecheck(ast)

	// Generate bytecode
	ctx := marigold.CodegenWithContext(ast)

	// Convert to blockchain instructions
	instructions := make([]blockchain.Instruction, len(ctx.Instructions))
	for i, instr := range ctx.Instructions {
		instructions[i] = blockchain.Instruction{
			Opcode: instr.Opcode,
			Value:  instr.Value,
		}
	}

	// Get deployer state for nonce
	deployerState := getAccountState(t, nodeAddr, deployerPubKey)

	deployTx := blockchain.Transaction{
		From:         deployerPubKey,
		To:           blockchain.PublicKey{}, // Zero address for contract deployment
		Amount:       0,
		Nonce:        deployerState.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: instructions,
		GasLimit:     50000,
		GasPrice:     100,
		Data:         nil,
	}
	deployTx.Signature = deployTx.GetSignature(deployerPriv)

	// Mine block with deployment
	block2 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&deployTx})
	submitBlock(t, nodeAddr, block2)
	time.Sleep(200 * time.Millisecond)

	// Find the contract address from the chain state
	var contractAddress blockchain.PublicKey
	allStates, _ := getChainState(t, nodeAddr)
	for addr, state := range allStates {
		if len(state.Instructions) > 0 && addr != deployerPubKey {
			contractAddress = addr
			break
		}
	}

	t.Logf("Contract address: %x", contractAddress)

	// Fund user account
	deployerState2 := getAccountState(t, nodeAddr, deployerPubKey)
	fundUserTx := blockchain.Transaction{
		From:         deployerPubKey,
		To:           userPubKey,
		Amount:       10000000,
		Nonce:        deployerState2.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: nil,
		GasLimit:     21000,
		GasPrice:     150,
		Data:         nil,
	}
	fundUserTx.Signature = fundUserTx.GetSignature(deployerPriv)

	block3 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&fundUserTx})
	submitBlock(t, nodeAddr, block3)
	time.Sleep(200 * time.Millisecond)

	// Test 1: Call with "transfer" command - should emit 999, then 111
	t.Log("Test 1: Calling with exact 'transfer' command")

	userState := getAccountState(t, nodeAddr, userPubKey)
	transferTx := blockchain.Transaction{
		From:         userPubKey,
		To:           contractAddress,
		Amount:       0,
		Nonce:        userState.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: nil,
		GasLimit:     50000,
		GasPrice:     100,
		Data:         []byte("transfer"), // Exact match
	}
	transferTx.Signature = transferTx.GetSignature(userPriv)

	block4 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&transferTx})
	submitBlock(t, nodeAddr, block4)
	time.Sleep(200 * time.Millisecond)

	t.Log("Should have seen emit 999 (about to compare) and emit 111 (transfer match)")

	// Test 2: Call with "transfers" command - should emit 999, then 222
	t.Log("Test 2: Calling with 'transfers' command (should not match)")

	userState2 := getAccountState(t, nodeAddr, userPubKey)
	transfersTx := blockchain.Transaction{
		From:         userPubKey,
		To:           contractAddress,
		Amount:       0,
		Nonce:        userState2.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: nil,
		GasLimit:     50000,
		GasPrice:     100,
		Data:         []byte("transfers"), // Should not match
	}
	transfersTx.Signature = transfersTx.GetSignature(userPriv)

	block5 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&transfersTx})
	submitBlock(t, nodeAddr, block5)
	time.Sleep(200 * time.Millisecond)

	t.Log("Should have seen emit 999 (about to compare) and emit 222 (no match)")

	t.Log("========== SIMPLE TRANSFER TEST COMPLETED ==========")
}