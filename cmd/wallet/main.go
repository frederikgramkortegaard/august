package main

import (
	"august/blockchain"
	"august/node/queryapi"
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
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
		fmt.Println("Error: expected a subcommand (balance or send)")
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
		return fmt.Errorf("HTTP error getting balance: %s", resp.Status)
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
		Signature: blockchain.Signature{},
		Nonce:     nextNonce,
		Timestamp: uint64(time.Now().Unix()),
	}

	blockchain.SignTransaction(&tsx, *priv)

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
		return fmt.Errorf("HTTP error: %s", resp.Status)
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
