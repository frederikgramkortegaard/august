package main

import (
	"august/avm"
	"august/blockchain"
	"august/node/queryapi"
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"
)

func main() {
	// Command line flags
	nodeAddr := flag.String("node", "localhost:8080", "Node HTTP API address to send transactions to")
	privKey := flag.String("privkey", "", "Private key encoded as Hex")
	flag.Parse()

	if *privKey == "" {
		log.Fatal("No private key was supplied")
	}

	// Decode hex → bytes
	privBytes, err := hex.DecodeString(*privKey)
	if err != nil {
		log.Fatal(err)
	}

	// Create ed25519 private key
	priv := ed25519.PrivateKey(privBytes)
	// Derive public key
	pub := priv.Public().(ed25519.PublicKey)

	log.Printf("\tWallet starting, address: %x", pub)
	log.Printf("\tConnecting to node: %s", *nodeAddr)

	// Remaining args after global flags
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Error: expected a subcommand (balance, send, or deploy)")
		os.Exit(1)
	}

	switch args[0] {
	case "balance":
		balanceCmd := flag.NewFlagSet("balance", flag.ExitOnError)
		// add balance-specific flags here if needed
		balanceCmd.Parse(args[1:])
		err := checkBalance(&pub, *nodeAddr)
		if err != nil {
			fmt.Println(err)
		}

	case "send":
		sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
		amount := sendCmd.Uint64("amount", 0, "Amount to send")
		to := sendCmd.String("to", "", "Recipient")
		sendCmd.Parse(args[1:])

		if *amount <= 0 || *to == "" {
			fmt.Println("Error: --amount and --to are required for send")
			sendCmd.Usage()
			os.Exit(1)
		}

		toPub := convertHexStringToPubKey(*to)
		err := sendMoney(&pub, &priv, *amount, &toPub, *nodeAddr)
		if err != nil {
			fmt.Println(err)
		}

	case "deploy":
		deployCmd := flag.NewFlagSet("deploy", flag.ExitOnError)
		amount := deployCmd.Uint64("amount", 0, "Amount to send to contract")
		gasLimit := deployCmd.Uint64("gas-limit", 50000, "Gas limit for deployment")
		gasPrice := deployCmd.Uint64("gas-price", 100, "Gas price")
		deployCmd.Parse(args[1:])

		err := deployContract(&pub, &priv, *amount, *gasLimit, *gasPrice, *nodeAddr)
		if err != nil {
			fmt.Println(err)
		}

	case "call":
		callCmd := flag.NewFlagSet("call", flag.ExitOnError)
		contract := callCmd.String("contract", "", "Contract address to call")
		amount := callCmd.Uint64("amount", 0, "Amount to send to contract")
		gasLimit := callCmd.Uint64("gas-limit", 50000, "Gas limit for call")
		gasPrice := callCmd.Uint64("gas-price", 100, "Gas price")
		callCmd.Parse(args[1:])

		if *contract == "" {
			fmt.Println("Error: --contract is required for call")
			callCmd.Usage()
			os.Exit(1)
		}

		contractPub := convertHexStringToPubKey(*contract)
		err := callContract(&pub, &priv, *amount, &contractPub, *gasLimit, *gasPrice, *nodeAddr)
		if err != nil {
			fmt.Println(err)
		}

	default:
		fmt.Println("Unknown subcommand:", args[0])
		os.Exit(1)
	}

}

func convertHexStringToPubKey(hexVal string) ed25519.PublicKey {
	// Decode hex string
	pubBytes, err := hex.DecodeString(hexVal)
	if err != nil {
		fmt.Println("Invalid hex:", err)
		return ed25519.PublicKey{}
	}

	// Validate size
	if len(pubBytes) != ed25519.PublicKeySize {
		fmt.Printf("Invalid public key size: got %d, expected %d\n", len(pubBytes), ed25519.PublicKeySize)
		return ed25519.PublicKey{}
	}

	// Convert to ed25519.PublicKey
	pubKey := ed25519.PublicKey(pubBytes)

	fmt.Printf("Public Key: %x\n", pubKey)
	return pubKey
}

func checkBalance(pubKey *ed25519.PublicKey, nodeaddr string) error {

	keyAsHex := hex.EncodeToString(*pubKey)

	// Attempt to send a transaction
	// Send HTTP POST request
	url := fmt.Sprintf("http://%s/balance/%s", nodeaddr, keyAsHex)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to check balance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Read response
	var response queryapi.BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Exists {
		log.Printf("\tBalance: %d", response.Balance)
		log.Printf("\tNonce: %d", response.Nonce)
	} else {
		log.Printf("\tAccount does not exist (balance: 0)")
	}
	return nil
}
func sendMoney(pub *ed25519.PublicKey, priv *ed25519.PrivateKey, amount uint64, to *ed25519.PublicKey, nodeaddr string) error {

	// First, get current balance and nonce
	keyAsHex := hex.EncodeToString(*pub)
	url := fmt.Sprintf("http://%s/balance/%s", nodeaddr, keyAsHex)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the error response body to get the actual error message
		bodyBytes := make([]byte, 1024) // Read up to 1KB of error message
		n, _ := resp.Body.Read(bodyBytes)
		errorBody := string(bodyBytes[:n])
		return fmt.Errorf("HTTP error getting balance %s: %s", resp.Status, errorBody)
	}

	var balanceResp queryapi.BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		return fmt.Errorf("failed to decode balance response: %w", err)
	}

	if !balanceResp.Exists {
		return fmt.Errorf("account does not exist")
	}

	if balanceResp.Balance < amount {
		return fmt.Errorf("insufficient balance: have %d, need %d", balanceResp.Balance, amount)
	}

	// Create transaction with correct nonce (current nonce + 1)
	nextNonce := balanceResp.Nonce + 1
	log.Printf("Creating transaction: amount=%d, current_nonce=%d, using_nonce=%d",
		amount, balanceResp.Nonce, nextNonce)

	tsx := blockchain.Transaction{
		From:      blockchain.PublicKey(*pub),
		To:        blockchain.PublicKey(*to),
		Amount:    amount,
		GasLimit:  25000, // Standard gas limit for transfers
		GasPrice:  100,   // Standard gas price
		Signature: blockchain.Signature{},
		Nonce:     nextNonce,
		Timestamp: uint64(time.Now().Unix()),
	}

	tsx.Signature = tsx.GetSignature(*priv)

	// Marshal transaction to JSON
	txJSON, err := json.Marshal(tsx)
	if err != nil {
		log.Printf("Failed to marshal transaction: %v", err)
		return err
	}

	url = fmt.Sprintf("http://%s/submit-transaction", nodeaddr)
	resp, err = http.Post(url, "application/json", bytes.NewBuffer(txJSON))
	if err != nil {
		log.Printf("Failed to submit transaction: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the error response body to get the actual error message
		bodyBytes := make([]byte, 1024) // Read up to 1KB of error message
		n, _ := resp.Body.Read(bodyBytes)
		errorBody := string(bodyBytes[:n])
		return fmt.Errorf("Transaction submission failed %s: %s", resp.Status, errorBody)
	}

	// Read response
	var response queryapi.SubmitTransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("Transaction submitted successfully!")
	log.Printf("Status: %s", response.Status)
	log.Printf("Hash: %s", response.Hash)
	return nil
}

// GetContract returns hardcoded contract bytecode that you can modify for testing
// This is a simple storage contract that stores a value at address 1 and increments it
func GetContract() ([]avm.Instruction, []avm.Instruction) {
	// Initialization code - runs once when contract is deployed
	initInstructions := []avm.Instruction{
		{Opcode: avm.PUSH, Value: big.NewInt(42)}, // Initial value
		{Opcode: avm.PUSH, Value: big.NewInt(1)},  // Storage address
		{Opcode: avm.PSTORE},                      // Store 42 at address 1
	}

	// Runtime code - runs every time contract is called
	runtimeInstructions := []avm.Instruction{
		{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Storage address
		{Opcode: avm.PLOAD},                      // Load current value from address 1
		{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Increment amount
		{Opcode: avm.ADD},                        // Add 1 to current value
		{Opcode: avm.DUP},                        // Duplicate result for emit
		{Opcode: avm.PUSH, Value: big.NewInt(1)}, // Storage address
		{Opcode: avm.SWAP, Param: 1}, // Swap to get incremented value on top
		{Opcode: avm.PSTORE},                     // Store incremented value at address 1
		{Opcode: avm.EMIT},                       // Emit the new value
	}

	return initInstructions, runtimeInstructions
}

func deployContract(pub *ed25519.PublicKey, priv *ed25519.PrivateKey, amount, gasLimit, gasPrice uint64, nodeaddr string) error {
	// Get current balance and nonce
	keyAsHex := hex.EncodeToString(*pub)
	url := fmt.Sprintf("http://%s/balance/%s", nodeaddr, keyAsHex)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the error response body to get the actual error message
		bodyBytes := make([]byte, 1024) // Read up to 1KB of error message
		n, _ := resp.Body.Read(bodyBytes)
		errorBody := string(bodyBytes[:n])
		return fmt.Errorf("HTTP error getting balance %s: %s", resp.Status, errorBody)
	}

	var balanceResp queryapi.BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		return fmt.Errorf("failed to decode balance response: %w", err)
	}

	if !balanceResp.Exists {
		return fmt.Errorf("account does not exist")
	}

	// Calculate total cost including gas
	maxGasCost := gasLimit * gasPrice
	totalCost := amount + maxGasCost
	if balanceResp.Balance < totalCost {
		return fmt.Errorf("insufficient balance: have %d, need %d (amount=%d, maxGas=%d)",
			balanceResp.Balance, totalCost, amount, maxGasCost)
	}

	// Get contract code
	initInstructions, runtimeInstructions := GetContract()

	// Create contract deployment transaction
	nextNonce := balanceResp.Nonce + 1
	log.Printf("Deploying contract: amount=%d, gasLimit=%d, gasPrice=%d, nonce=%d",
		amount, gasLimit, gasPrice, nextNonce)

	tsx := blockchain.Transaction{
		From:             blockchain.PublicKey(*pub),
		To:               blockchain.PublicKey{}, // Empty address = contract deployment
		Amount:           amount,
		GasLimit:         gasLimit,
		GasPrice:         gasPrice,
		Signature:        blockchain.Signature{},
		Nonce:            nextNonce,
		Timestamp:        uint64(time.Now().Unix()),
		Instructions:     runtimeInstructions, // Runtime code
		InitInstructions: initInstructions,    // Initialization code
	}

	tsx.Signature = tsx.GetSignature(*priv)

	// Submit transaction
	txJSON, err := json.Marshal(tsx)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	url = fmt.Sprintf("http://%s/submit-transaction", nodeaddr)
	resp, err = http.Post(url, "application/json", bytes.NewBuffer(txJSON))
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the error response body to get the actual error message
		bodyBytes := make([]byte, 1024) // Read up to 1KB of error message
		n, _ := resp.Body.Read(bodyBytes)
		errorBody := string(bodyBytes[:n])
		return fmt.Errorf("Contract deployment failed %s: %s", resp.Status, errorBody)
	}

	// Read response
	var response queryapi.SubmitTransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("Contract deployment submitted successfully!")
	log.Printf("Status: %s", response.Status)
	log.Printf("Transaction Hash: %s", response.Hash)
	log.Printf("Contract will be deployed when transaction is mined")
	return nil
}

func callContract(pub *ed25519.PublicKey, priv *ed25519.PrivateKey, amount uint64, contract *ed25519.PublicKey, gasLimit, gasPrice uint64, nodeaddr string) error {
	// Get current balance and nonce
	keyAsHex := hex.EncodeToString(*pub)
	url := fmt.Sprintf("http://%s/balance/%s", nodeaddr, keyAsHex)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the error response body to get the actual error message
		bodyBytes := make([]byte, 1024) // Read up to 1KB of error message
		n, _ := resp.Body.Read(bodyBytes)
		errorBody := string(bodyBytes[:n])
		return fmt.Errorf("HTTP error getting balance %s: %s", resp.Status, errorBody)
	}

	var balanceResp queryapi.BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		return fmt.Errorf("failed to decode balance response: %w", err)
	}

	if !balanceResp.Exists {
		return fmt.Errorf("account does not exist")
	}

	// Calculate total cost including gas
	maxGasCost := gasLimit * gasPrice
	totalCost := amount + maxGasCost
	if balanceResp.Balance < totalCost {
		return fmt.Errorf("insufficient balance: have %d, need %d (amount=%d, maxGas=%d)",
			balanceResp.Balance, totalCost, amount, maxGasCost)
	}

	// Create contract call transaction
	nextNonce := balanceResp.Nonce + 1
	log.Printf("Calling contract: contract=%x, amount=%d, gasLimit=%d, gasPrice=%d, nonce=%d",
		*contract, amount, gasLimit, gasPrice, nextNonce)

	tsx := blockchain.Transaction{
		From:      blockchain.PublicKey(*pub),
		To:        blockchain.PublicKey(*contract),
		Amount:    amount,
		GasLimit:  gasLimit,
		GasPrice:  gasPrice,
		Signature: blockchain.Signature{},
		Nonce:     nextNonce,
		Timestamp: uint64(time.Now().Unix()),
	}

	tsx.Signature = tsx.GetSignature(*priv)

	// Submit transaction
	txJSON, err := json.Marshal(tsx)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	url = fmt.Sprintf("http://%s/submit-transaction", nodeaddr)
	resp, err = http.Post(url, "application/json", bytes.NewBuffer(txJSON))
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the error response body to get the actual error message
		bodyBytes := make([]byte, 1024) // Read up to 1KB of error message
		n, _ := resp.Body.Read(bodyBytes)
		errorBody := string(bodyBytes[:n])
		return fmt.Errorf("Contract call failed %s: %s", resp.Status, errorBody)
	}

	// Read response
	var response queryapi.SubmitTransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("Contract call submitted successfully!")
	log.Printf("Status: %s", response.Status)
	log.Printf("Transaction Hash: %s", response.Hash)
	return nil
}
