package main

import (
	"august/blockchain"
	"august/config"
	_ "august/execution" // Import for side effects (registers executor)
	"august/node"
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	// Command line flags
	nodeAddr := flag.String("node", "localhost:8080", "Node HTTP API address to mine for")
	minerID := flag.String("id", "", "Miner ID (auto-generated if not provided)")
	privKey := flag.String("privkey", "", "Private key encoded as Hex")
	maxBlocks := flag.Int("maxblocks", 0, "Maximum number of blocks to mine (0 = unlimited)")
	flag.Parse()

	if *privKey == "" {
		log.Fatal("No private key was supplied")
	}

	// Auto-generate miner ID if not provided
	if *minerID == "" {
		*minerID = fmt.Sprintf("miner-%d", time.Now().Unix()%10000)
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

	log.Printf("%s\tMiner starting, address: %x", *minerID, pub)
	log.Printf("%s\tConnecting to node: %s", *minerID, *nodeAddr)

	// Create miner
	simpleMiner := &SimpleMiner{
		ID:           *minerID,
		NodeAddr:     *nodeAddr,
		MinerAddress: blockchain.PublicKey(pub),
		PrivateKey:   priv,
		MaxBlocks:    *maxBlocks,
	}

	// Start mining loop
	simpleMiner.StartMining()
}

type SimpleMiner struct {
	ID           string
	NodeAddr     string
	MinerAddress blockchain.PublicKey
	PrivateKey   ed25519.PrivateKey
	MaxBlocks    int
}

func (m *SimpleMiner) StartMining() {
	log.Printf("%s\tStarting mining loop...", m.ID)
	if m.MaxBlocks > 0 {
		log.Printf("%s\tMining limited to %d blocks", m.ID, m.MaxBlocks)
	}

	blocksMinedCount := 0
	for {
		// Check if we've reached the maximum blocks limit
		if m.MaxBlocks > 0 && blocksMinedCount >= m.MaxBlocks {
			log.Printf("%s\tReached maximum blocks limit (%d), stopping", m.ID, m.MaxBlocks)
			return
		}
		// Get mining info from node
		chainInfo, err := m.getChainInfo()
		if err != nil {
			log.Printf("%s\tFailed to get chain info: %v", m.ID, err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("%s\tCurrent chain: height %d, head %x", m.ID, chainInfo.Height, chainInfo.Hash[:8])

		// Create and mine block
		block, err := m.createAndMineBlock(chainInfo)
		if err != nil {
			log.Printf("%s\tFailed to create block: %v", m.ID, err)
			time.Sleep(1 * time.Second)
			continue
		}

		blockHash := block.Header.GetHash()
		log.Printf("%s\tMined block %x (height %d, nonce %d)",
			m.ID, blockHash[:8], block.Header.Height, block.Header.Nonce)

		// Submit block to node
		if err := m.submitBlock(&block); err != nil {
			log.Printf("%s\tFailed to submit block: %v", m.ID, err)
		} else {
			log.Printf("%s\tBlock submitted successfully!", m.ID)
			blocksMinedCount++
			log.Printf("%s\tBlocks mined: %d", m.ID, blocksMinedCount)
		}

		// Check if we've reached the maximum blocks limit after successful mining
		if m.MaxBlocks > 0 && blocksMinedCount >= m.MaxBlocks {
			log.Printf("%s\tReached maximum blocks limit (%d), stopping", m.ID, m.MaxBlocks)
			return
		}

		// Small delay before next mining attempt
		time.Sleep(30 * time.Second)
	}
}

func (m *SimpleMiner) getChainInfo() (*blockchain.ChainHead, error) {
	url := fmt.Sprintf("http://%s/chain-info", m.NodeAddr)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	var chainHead blockchain.ChainHead
	if err := json.NewDecoder(resp.Body).Decode(&chainHead); err != nil {
		return nil, fmt.Errorf("failed to decode chain info: %w", err)
	}

	return &chainHead, nil
}

func (m *SimpleMiner) createAndMineBlock(chainInfo *blockchain.ChainHead) (blockchain.Block, error) {

	// Get pending transactions from mempool
	mempoolTxs, err := m.getMempoolTransactions()
	if err != nil {
		log.Printf("%s\tWarning: Failed to get mempool transactions: %v", m.ID, err)
		mempoolTxs = []blockchain.Transaction{} // Continue with empty mempool
	}

	// Get current chain state for transaction validation
	currentStates, err := m.getChainState()
	if err != nil {
		log.Printf("%s\tWarning: Failed to get chain state: %v", m.ID, err)
		currentStates = make(map[blockchain.PublicKey]*blockchain.AccountState) // Continue with empty state
	}

	log.Printf("%s\tIncluding %d transactions from mempool", m.ID, len(mempoolTxs))
	log.Printf("%s\tCurrent chain state has %d accounts", m.ID, len(currentStates))

	// Calculate total gas fees by simulating transaction execution
	var totalGasFees uint64
	for _, tx := range mempoolTxs {
		// Skip coinbase transactions
		if tx.From == (blockchain.PublicKey{}) {
			continue
		}

		// Create a copy of states for simulation
		tempStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
		for pubKey, state := range currentStates {
			if state != nil {
				tempStates[pubKey] = &blockchain.AccountState{
					Address:      state.Address,
					Balance:      state.Balance,
					Nonce:        state.Nonce,
					Instructions: state.Instructions,
					Persistent:   make(map[string]string),
					StorageRoot:  state.StorageRoot,
					CodeHash:     state.CodeHash,
				}
				for k, v := range state.Persistent {
					tempStates[pubKey].Persistent[k] = v
				}
			}
		}

		// Try to execute the transaction to see if it's valid and get gas usage
		gasUsed, err := blockchain.ApplyTransaction(&tx, tempStates)
		if err != nil {
			log.Printf("%s\tSkipping invalid transaction: %v", m.ID, err)
			continue
		}

		gasFee := gasUsed * tx.GasPrice
		totalGasFees += gasFee
	}

	log.Printf("%s\tCalculated total gas fees: %d", m.ID, totalGasFees)

	// Create coinbase transaction with block reward + gas fees
	coinbaseAmount := config.BlockReward + totalGasFees
	coinbase := blockchain.Transaction{
		From:             blockchain.PublicKey{}, // Empty for coinbase
		To:               m.MinerAddress,
		Amount:           coinbaseAmount,
		Nonce:            0,
		Timestamp:        uint64(time.Now().Unix()),
		Signature:        blockchain.Signature{}, // Empty for coinbase
		ChainID:          config.MainnetChainID,
		Instructions:     nil,
		InitInstructions: nil,
		GasLimit:         0, // Coinbase doesn't consume gas
		GasPrice:         0, // Coinbase doesn't pay gas
	}

	// Use the actual chain head hash
	previousHash := chainInfo.Hash

	// Create mining parameters
	params := blockchain.BlockCreationParams{
		Version:       1,
		PreviousHash:  previousHash,
		Height:        chainInfo.Height + 1,
		PreviousWork:  chainInfo.TotalWork,
		Coinbase:      coinbase,
		Transactions:  mempoolTxs, // Include mempool transactions
		Timestamp:     uint64(time.Now().Unix()),
		TargetBits:    config.TestTargetCompact, // Use easy difficulty for testing
		CurrentStates: currentStates,            // Include current account states for validation
	}

	// Mine the block (this will take some time!)
	log.Printf("%s\tMining block at height %d...", m.ID, params.Height)
	return blockchain.NewBlock(params)
}

func (m *SimpleMiner) getMempoolTransactions() ([]blockchain.Transaction, error) {
	url := fmt.Sprintf("http://%s/mempool", m.NodeAddr)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get mempool: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	var mempoolResp node.MempoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&mempoolResp); err != nil {
		return nil, fmt.Errorf("failed to decode mempool response: %w", err)
	}

	return mempoolResp.Transactions, nil
}

func (m *SimpleMiner) getChainState() (map[blockchain.PublicKey]*blockchain.AccountState, error) {
	url := fmt.Sprintf("http://%s/chain-state", m.NodeAddr)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	var chainStateResp node.ChainStateResponse
	if err := json.NewDecoder(resp.Body).Decode(&chainStateResp); err != nil {
		return nil, fmt.Errorf("failed to decode chain state response: %w", err)
	}

	// Convert hex string keys back to PublicKey
	accountStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
	for keyHex, state := range chainStateResp.AccountStates {
		keyBytes, err := hex.DecodeString(keyHex)
		if err != nil {
			log.Printf("%s\tWarning: invalid public key hex %s: %v", m.ID, keyHex, err)
			continue
		}

		if len(keyBytes) != 32 {
			log.Printf("%s\tWarning: invalid public key length %d for %s", m.ID, len(keyBytes), keyHex)
			continue
		}

		var pubKey blockchain.PublicKey
		copy(pubKey[:], keyBytes)
		accountStates[pubKey] = state
	}

	return accountStates, nil
}

func (m *SimpleMiner) submitBlock(block *blockchain.Block) error {
	// Marshal block to JSON
	blockData, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	// Send HTTP POST request
	url := fmt.Sprintf("http://%s/submit-block", m.NodeAddr)
	resp, err := http.Post(url, "application/json", bytes.NewReader(blockData))
	if err != nil {
		return fmt.Errorf("failed to submit block: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Read response
	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("%s\tReceived response: %s", m.ID, response["status"])
	return nil
}
