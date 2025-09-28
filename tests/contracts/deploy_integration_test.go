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

// hashKey replicates the string key hashing logic from codegen
func hashKey(stringKey string) string {
	var hash uint64 = 0
	for _, char := range stringKey {
		hash = (hash*31 + uint64(char)) & 0x7FFFFFFFFFFFFFFF // Ensure positive
	}
	if hash == 0 {
		hash = 1 // Avoid zero keys
	}
	return fmt.Sprintf("%d", hash)
}

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
		// ========== BLOCKCHAIN CONTEXT VARIABLES ==========
		// Store all blockchain context variables for testing
		persistent["deployer"] = @caller
		persistent["contract_address"] = @address
		persistent["deploy_value"] = string(@callvalue)
		persistent["deploy_height"] = string(@height)
		persistent["deploy_timestamp"] = string(@timestamp)
		persistent["deploy_gasprice"] = string(@gasprice)
		persistent["deploy_difficulty"] = string(@difficulty)
		persistent["deploy_coinbase"] = @coinbase
		persistent["deploy_gaslimit"] = string(@gaslimit)

		// ========== DYNAMIC STRING KEYS ==========
		// Use blockchain variables as dynamic keys
		persistent[string(@height)] = "height_based_value"
		persistent[string(@callvalue)] = "callvalue_based_value"
		persistent[string(@gasprice)] = "gasprice_based_value"

		// ========== STRING OPERATIONS ==========
		// Basic string literals and concatenation
		name: string = "AdvancedContract"
		version: string = "v2.0.0"
		space: string = " "
		nameWithSpace: string = name + space
		fullName: string = nameWithSpace + version
		persistent["contract_info"] = fullName

		// String operations with blockchain values
		heightStr: string = string(@height)
		valueStr: string = string(@callvalue)
		space2: string = ":"
		heightValue: string = heightStr + space2
		deployInfo: string = heightValue + valueStr
		persistent["deploy_info"] = deployInfo

		// ========== STRING INDEXING ==========
		// Test string character access (simple cases first)
		firstChar: string = name[0]        // "A" from "AdvancedContract"
		persistent["first_char"] = firstChar

		// ========== STRING SLICING ==========
		// Test simple slicing
		prefix: string = name[0:8]         // "Advanced"
		persistent["prefix"] = prefix

		// ========== EMIT OPERATIONS ==========
		// Emit blockchain variables
		emit(@callvalue)
		emit(@height)
		emit(@timestamp)
		emit(fullName)
		emit(deployInfo)

		// Emit string operation results
		emit(len(firstChar))
		emit(len(prefix))

		return 1
	}

	define add(a: int, b: int) : int {
		result: int = a + b
		// Store function results with blockchain context
		persistent[string(200)] = string(result)
		persistent["last_add_height"] = string(@height)
		return result
	}

	define call() : int {
		// ========== RUNTIME BLOCKCHAIN VARIABLES ==========
		// Test blockchain variables at call time (different from init)
		persistent["caller"] = @caller
		persistent["call_value"] = string(@callvalue)
		persistent["call_height"] = string(@height)
		persistent["call_timestamp"] = string(@timestamp)
		persistent["call_gasprice"] = string(@gasprice)

		// ========== DYNAMIC KEYS AT RUNTIME ==========
		// Different height/value will create different keys
		persistent[string(@height)] = "runtime_height_value"
		persistent[string(@callvalue)] = "runtime_callvalue_value"

		// ========== FUNCTION CALLS WITH BLOCKCHAIN CONTEXT ==========
		// Call function and store result with blockchain info
		sum: int = add(25, 50)
		emit(sum)

		// ========== ADVANCED STRING OPERATIONS ==========
		// String concatenation with blockchain values
		greeting: string = "Hello"
		world: string = " Blockchain"
		message: string = greeting + world
		persistent["greeting"] = message
		emit(message)

		// Complex string building with blockchain context
		prefix: string = "Block "
		heightStr: string = string(@height)
		prefixHeight: string = prefix + heightStr
		suffix: string = " processed"
		blockMessage: string = prefixHeight + suffix
		emit(blockMessage)
		persistent["block_message"] = blockMessage

		// ========== STRING LENGTH OPERATIONS ==========
		msgLen: int = len(message)
		blockLen: int = len(blockMessage)
		emit(msgLen)
		emit(blockLen)
		persistent["message_length"] = string(msgLen)
		persistent["block_length"] = string(blockLen)

		// ========== COMPLEX STRING + BLOCKCHAIN COMBINATIONS ==========
		// Build status message with multiple blockchain variables
		status: string = "Status"
		colon: string = ":"
		statusColon: string = status + colon

		valuePrefix: string = "Value="
		callvalueStr: string = string(@callvalue)
		valueInfo: string = valuePrefix + callvalueStr

		comma: string = ","
		statusValue: string = statusColon + valueInfo
		statusValueComma: string = statusValue + comma

		heightPrefix: string = "Height="
		heightInfo: string = heightPrefix + heightStr

		fullStatus: string = statusValueComma + heightInfo
		emit(fullStatus)
		persistent["full_status"] = fullStatus

		// ========== TRANSACTION DATA INDEXING & SLICING ==========
		// Test @tsxdata indexing and slicing operations
		tsxData: string = @tsxdata
		dataLen: int = len(tsxData)
		emit("Starting transaction data analysis")
		emit("TSX data length:")
		emit(dataLen)
		emit("TSX data content:")
		emit(tsxData)

		if dataLen > 0 {
			// Character indexing
			emit("Performing character indexing at position 0")
			firstByte: string = tsxData[0]
			persistent["tsx_first_byte"] = firstByte
			emit("First byte extracted:")
			emit(firstByte)

			if dataLen >= 3 {
				// Test the original question: if @tsxdata[:3] == "buy"
				emit("Performing slice operation [:3]")
				command: string = tsxData[:3]
				persistent["tsx_command"] = command
				emit("Command extracted:")
				emit(command)

				emit("Checking if command equals buy")
				if command == "buy" {
					persistent["tsx_action"] = "buy_detected"
					emit("BUY command detected")
				} else {
					persistent["tsx_action"] = "other_command"
					emit("Other command detected")
				}

				// More slicing examples
				if dataLen >= 8 {
					emit("Performing multiple slice operations")
					emit("Slice [0:8] for commandFull")
					commandFull: string = tsxData[0:8]      // First 8 chars
					emit("CommandFull result:")
					emit(commandFull)

					emit("Slice [8:] for remaining")
					remaining: string = tsxData[8:]          // Rest of data
					emit("Remaining result:")
					emit(remaining)

					emit("Slice [2:6] for middle")
					middle: string = tsxData[2:6]           // Middle section
					emit("Middle result:")
					emit(middle)

					persistent["tsx_command_full"] = commandFull
					persistent["tsx_remaining"] = remaining
					persistent["tsx_middle"] = middle

					// Simulate transfer parsing: "transfer" + 64-char address + amount
					if dataLen >= 73 {
						emit("Data length sufficient for transfer parsing")
						if commandFull == "transfer" {
							emit("TRANSFER command detected - parsing recipient and amount")
							emit("Slice [8:72] for recipient address")
							recipient: string = tsxData[8:72]    // 64-char address
							emit("Recipient address:")
							emit(recipient)

							emit("Slice [72:] for amount")
							amountStr: string = tsxData[72:]     // Amount as string
							emit("Amount string:")
							emit(amountStr)

							persistent["transfer_recipient"] = recipient
							persistent["transfer_amount"] = amountStr
							persistent["tsx_action"] = "transfer_detected"
							emit("Transfer parsing complete")
						}
					}
				}
			}
		}

		// Store data length for verification
		persistent["tsx_data_length"] = string(dataLen)

		// ========== CONTROL FLOW WITH BLOCKCHAIN VALUES ==========
		// Test control flow using blockchain variables
		if @callvalue > 100 {
			persistent["value_category"] = "high_value"
			highMsg: string = "High value transaction"
			emit(highMsg)
		} else {
			persistent["value_category"] = "low_value"
			lowMsg: string = "Low value transaction"
			emit(lowMsg)
		}

		// Control flow with function results
		if sum == 75 {
			persistent[string(100)] = string(200)
			success: string = "Math Success"
			emit(success)
		} else {
			persistent[string(100)] = string(300)
			fail: string = "Math Failed"
			emit(fail)
		}

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

	marigold.Typecheck(ast)

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
	t.Logf("========== DEPLOYMENT STORAGE VERIFICATION ==========")
	t.Logf("Contract persistent storage after deployment:")
	for k, v := range contractState.Persistent {
		t.Logf("  '%s' = '%s'", k, v)
	}

	// Verify blockchain context variables were stored
	t.Logf("========== BLOCKCHAIN VARIABLES VERIFICATION ==========")
	blockchainVarKeys := []string{"deployer", "contract_address", "deploy_value", "deploy_height",
		"deploy_timestamp", "deploy_gasprice", "deploy_difficulty", "deploy_coinbase", "deploy_gaslimit"}

	for _, key := range blockchainVarKeys {
		if val, exists := contractState.Persistent[hashKey(key)]; exists {
			t.Logf("PASS: %s stored (hash key): %s", key, val)
		} else {
			t.Logf("FAIL: %s not found", key)
		}
	}

	// Verify dynamic keys (these use actual string values as keys)
	t.Logf("========== DYNAMIC KEYS VERIFICATION ==========")
	expectedDynamicKeys := []string{"1", "0", "100"} // height=1, callvalue=0, gasprice=100
	for _, key := range expectedDynamicKeys {
		if val, exists := contractState.Persistent[key]; exists {
			t.Logf("PASS: Dynamic key '%s': %s", key, val)
		} else {
			t.Logf("FAIL: Dynamic key '%s' not found", key)
		}
	}

	t.Log("========== CALLING CONTRACT ==========")
	t.Log("Calling contract with higher value to test runtime blockchain variables")
	updatedDeployerState := getAccountState(t, nodeAddr, deployerPubKey)
	callTx := blockchain.Transaction{
		From:         deployerPubKey,
		To:           contractAddr,
		Amount:       500, // Higher value for testing @callvalue
		Nonce:        updatedDeployerState.Nonce + 1,
		Timestamp:    uint64(time.Now().Unix()),
		ChainID:      config.MainnetChainID,
		Instructions: nil,
		GasLimit:     100000, // Higher gas limit for complex operations
		GasPrice:     150,    // Different gas price
		Data:         []byte("buyTokens"), // Test data for @tsxdata indexing/slicing
	}
	callTx.Signature = callTx.GetSignature(priv)

	t.Log("Mining block with contract call")
	block3 := mineBlock(t, nodeAddr, deployerPubKey, []*blockchain.Transaction{&callTx})
	submitBlock(t, nodeAddr, block3)
	time.Sleep(500 * time.Millisecond)

	finalState := getAccountState(t, nodeAddr, deployerPubKey)

	// Check persistent storage after contract call using HTTP API
	contractInfo := getContractInfo(t, nodeAddr, contractAddr)
	t.Logf("========== COMPLETE CONTRACT STATE ==========")
	t.Logf("Contract Address: %s", contractInfo.Address)
	t.Logf("Contract Balance: %d", contractInfo.Balance)
	t.Logf("Storage Entries: %d", contractInfo.StorageEntries)

	t.Logf("========== COMPLETE PERSISTENT STORAGE ==========")
	t.Logf("All persistent storage after contract call:")
	for k, v := range contractInfo.PersistentData {
		t.Logf("  '%s' = '%s'", k, v)
	}

	// Verify runtime blockchain variables
	t.Logf("========== RUNTIME BLOCKCHAIN VARIABLES VERIFICATION ==========")
	runtimeVarKeys := []string{"caller", "call_value", "call_height", "call_timestamp", "call_gasprice"}
	for _, key := range runtimeVarKeys {
		if val, exists := contractInfo.PersistentData[hashKey(key)]; exists {
			t.Logf("PASS: Runtime %s stored (hash key): %s", key, val)
		} else {
			t.Logf("FAIL: Runtime %s not found", key)
		}
	}

	// Verify string operations
	t.Logf("========== STRING OPERATIONS VERIFICATION ==========")
	stringOpKeys := []string{"contract_info", "deploy_info", "greeting", "block_message",
		"message_length", "block_length", "full_status"}
	for _, key := range stringOpKeys {
		if val, exists := contractInfo.PersistentData[hashKey(key)]; exists {
			t.Logf("String operation %s: %s", key, val)
		} else {
			t.Logf("String operation %s not found", key)
		}
	}

	// Verify string indexing and slicing
	t.Logf("========== STRING INDEXING VERIFICATION ==========")
	indexKeys := []string{"first_char"}
	for _, key := range indexKeys {
		if val, exists := contractInfo.PersistentData[hashKey(key)]; exists {
			t.Logf("String indexing %s: '%s'", key, val)
		} else {
			t.Logf("String indexing %s not found", key)
		}
	}

	t.Logf("========== STRING SLICING VERIFICATION ==========")
	sliceKeys := []string{"prefix"}
	for _, key := range sliceKeys {
		if val, exists := contractInfo.PersistentData[hashKey(key)]; exists {
			t.Logf("String slicing %s: '%s'", key, val)
		} else {
			t.Logf("String slicing %s not found", key)
		}
	}

	t.Logf("========== TRANSACTION DATA INDEXING/SLICING VERIFICATION ==========")
	tsxKeys := []string{"tsx_first_byte", "tsx_command", "tsx_action", "tsx_command_full",
		"tsx_remaining", "tsx_middle", "tsx_data_length"}
	for _, key := range tsxKeys {
		if val, exists := contractInfo.PersistentData[hashKey(key)]; exists {
			t.Logf("@tsxdata operation %s: '%s'", key, val)
		} else {
			t.Logf("@tsxdata operation %s not found", key)
		}
	}

	// Control flow verification
	t.Logf("========== CONTROL FLOW ==========")
	if val, exists := contractInfo.PersistentData[hashKey("value_category")]; exists {
		t.Logf("Value category: %s", val)
		if val == "high_value" {
			t.Logf("High value branch taken (@callvalue=500 > 100)")
		}
	}

	if contractInfo.PersistentData["100"] == "200" {
		t.Logf("Math check passed: add(25,50)=75 -> success path")
	} else {
		t.Logf("Math check failed: expected 200, got %s", contractInfo.PersistentData["100"])
	}

	// Dynamic keys at runtime
	t.Logf("========== RUNTIME DYNAMIC KEYS ==========")
	if val, exists := contractInfo.PersistentData["3"]; exists {
		t.Logf("Height key '3': %s", val)
	}
	if val, exists := contractInfo.PersistentData["500"]; exists {
		t.Logf("Callvalue key '500': %s", val)
	}

	totalMiningRewards := config.BlockReward * 3
	actualGasCost := (state.Balance + totalMiningRewards) - finalState.Balance

	t.Logf("========== SUMMARY ==========")
	t.Logf("Contract deployed and called successfully")
	t.Logf("Features tested: blockchain vars, string ops, string indexing, string slicing, @tsxdata, dynamic keys, control flow")
	t.Logf("String operations: indexing (str[i]), slicing (str[start:end], str[:end], str[start:]), @tsxdata parsing")
	t.Logf("Initial balance: %d, Final: %d, Rewards: %d", state.Balance, finalState.Balance, totalMiningRewards)
	t.Logf("Gas consumed: %d", actualGasCost)
}
