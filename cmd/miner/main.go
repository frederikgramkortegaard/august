package main

import (
	"august/blockchain"
	"august/miner"
	"august/node/queryapi"
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
	miner := &SimpleMiner{
		ID:           *minerID,
		NodeAddr:     *nodeAddr,
		MinerAddress: blockchain.PublicKey(pub),
		PrivateKey:   priv,
	}

	// Start mining loop
	miner.StartMining()
}

type SimpleMiner struct {
	ID           string
	NodeAddr     string
	MinerAddress blockchain.PublicKey
	PrivateKey   ed25519.PrivateKey
}

func (m *SimpleMiner) StartMining() {
	log.Printf("%s\tStarting mining loop...", m.ID)

	for {
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

		blockHash := blockchain.HashBlockHeader(&block.Header)
		log.Printf("%s\tMined block %x (height %d, nonce %d)",
			m.ID, blockHash[:8], block.Header.Height, block.Header.Nonce)

		// Submit block to node
		if err := m.submitBlock(&block); err != nil {
			log.Printf("%s\tFailed to submit block: %v", m.ID, err)
		} else {
			log.Printf("%s\tBlock submitted successfully!", m.ID)
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
	// Create coinbase transaction
	coinbase := blockchain.Transaction{
		From:      blockchain.PublicKey{}, // Empty for coinbase
		To:        m.MinerAddress,
		Amount:    blockchain.BlockReward,
		Fee:       0,
		Nonce:     0,
		Timestamp: uint64(time.Now().Unix()),
		Signature: blockchain.Signature{}, // Empty for coinbase
	}

	// Get pending transactions from mempool
	mempoolTxs, err := m.getMempoolTransactions()
	if err != nil {
		log.Printf("%s\tWarning: Failed to get mempool transactions: %v", m.ID, err)
		mempoolTxs = []blockchain.Transaction{} // Continue with empty mempool
	}

	log.Printf("%s\tIncluding %d transactions from mempool", m.ID, len(mempoolTxs))

	// Use the actual chain head hash
	previousHash := chainInfo.Hash

	// Create mining parameters
	params := miner.BlockCreationParams{
		Version:      1,
		PreviousHash: previousHash,
		Height:       chainInfo.Height + 1,
		PreviousWork: chainInfo.TotalWork,
		Coinbase:     coinbase,
		Transactions: mempoolTxs, // Include mempool transactions
		Timestamp:    uint64(time.Now().Unix()),
		TargetBits:   blockchain.TestTargetCompact, // Use easy difficulty for testing
	}

	// Mine the block (this will take some time!)
	log.Printf("%s\tMining block at height %d...", m.ID, params.Height)
	return miner.NewBlock(params)
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

	var mempoolResp queryapi.MempoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&mempoolResp); err != nil {
		return nil, fmt.Errorf("failed to decode mempool response: %w", err)
	}

	return mempoolResp.Transactions, nil
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
