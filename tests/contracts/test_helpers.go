package contracts

import (
	"august/blockchain"
	"august/config"
	"august/node"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func mineBlock(t *testing.T, nodeAddr string, beneficiary blockchain.PublicKey, txs []*blockchain.Transaction) *blockchain.Block {
	chainInfo, err := getChainInfo(t, nodeAddr)
	if err != nil {
		t.Fatalf("Failed to get chain info: %v", err)
	}

	currentStates, err := getChainState(t, nodeAddr)
	if err != nil {
		t.Fatalf("Failed to get chain state: %v", err)
	}

	timestamp := uint64(time.Now().Unix())
	if chainInfo.Height > 0 {
		lastBlock, err := getLastBlock(t, nodeAddr, chainInfo.Hash)
		if err == nil && timestamp <= lastBlock.Header.Timestamp {
			timestamp = lastBlock.Header.Timestamp + 1
		}
	}

	var totalGasFees uint64
	for _, tx := range txs {
		if tx.From == (blockchain.PublicKey{}) {
			continue
		}

		tempStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
		for pubKey, st := range currentStates {
			if st != nil {
				tempStates[pubKey] = &blockchain.AccountState{
					Address:      st.Address,
					Balance:      st.Balance,
					Nonce:        st.Nonce,
					Instructions: st.Instructions,
					Persistent:   make(map[string]string),
					StorageRoot:  st.StorageRoot,
					CodeHash:     st.CodeHash,
				}
				for k, v := range st.Persistent {
					tempStates[pubKey].Persistent[k] = v
				}
			}
		}

		gasUsed, err := blockchain.ApplyTransaction(tx, tempStates)
		if err != nil {
			continue
		}

		gasFee := gasUsed * tx.GasPrice
		totalGasFees += gasFee
	}

	coinbaseAmount := config.BlockReward + totalGasFees
	coinbase := blockchain.Transaction{
		From:         blockchain.PublicKey{},
		To:           beneficiary,
		Amount:       coinbaseAmount,
		Nonce:        0,
		Timestamp:    timestamp,
		Signature:    blockchain.Signature{},
		ChainID:      config.MainnetChainID,
		Instructions: nil,
		GasLimit:     0,
		GasPrice:     0,
	}

	txList := make([]blockchain.Transaction, len(txs))
	for i, tx := range txs {
		txList[i] = *tx
	}

	params := blockchain.BlockCreationParams{
		Version:       1,
		PreviousHash:  chainInfo.Hash,
		Height:        chainInfo.Height + 1,
		PreviousWork:  chainInfo.TotalWork,
		Coinbase:      coinbase,
		Transactions:  txList,
		Timestamp:     timestamp,
		TargetBits:    config.TestTargetCompact,
		CurrentStates: currentStates,
	}

	block, err := blockchain.NewBlock(params)
	if err != nil {
		t.Fatalf("Failed to mine block: %v", err)
	}

	return &block
}

func submitBlock(t *testing.T, nodeAddr string, block *blockchain.Block) {
	blockData, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal block: %v", err)
	}

	url := fmt.Sprintf("http://%s/submit-block", nodeAddr)
	resp, err := http.Post(url, "application/json", bytes.NewReader(blockData))
	if err != nil {
		t.Fatalf("Failed to submit block: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP error: %s", resp.Status)
	}

	time.Sleep(200 * time.Millisecond)
}

func getBalance(t *testing.T, nodeAddr string, pubKey blockchain.PublicKey) uint64 {
	addrHex := hex.EncodeToString(pubKey[:])
	url := fmt.Sprintf("http://%s/balance/%s", nodeAddr, addrHex)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP error: %s", resp.Status)
	}

	var balanceResp node.BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		t.Fatalf("Failed to decode balance response: %v", err)
	}

	return balanceResp.Balance
}

func getAccountState(t *testing.T, nodeAddr string, pubKey blockchain.PublicKey) *blockchain.AccountState {
	states, err := getChainState(t, nodeAddr)
	if err != nil {
		t.Fatalf("Failed to get chain state: %v", err)
	}

	state, exists := states[pubKey]
	if !exists {
		return &blockchain.AccountState{
			Address:      pubKey,
			Balance:      0,
			Nonce:        0,
			Instructions: nil,
			Persistent:   make(map[string]string),
		}
	}

	return state
}

func getChainInfo(t *testing.T, nodeAddr string) (*blockchain.ChainHead, error) {
	url := fmt.Sprintf("http://%s/chain-info", nodeAddr)
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

func getChainState(t *testing.T, nodeAddr string) (map[blockchain.PublicKey]*blockchain.AccountState, error) {
	url := fmt.Sprintf("http://%s/chain-state", nodeAddr)
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

	accountStates := make(map[blockchain.PublicKey]*blockchain.AccountState)
	for keyHex, state := range chainStateResp.AccountStates {
		keyBytes, err := hex.DecodeString(keyHex)
		if err != nil {
			continue
		}

		if len(keyBytes) != 32 {
			continue
		}

		var pubKey blockchain.PublicKey
		copy(pubKey[:], keyBytes)
		accountStates[pubKey] = state
	}

	return accountStates, nil
}

func getLastBlock(t *testing.T, nodeAddr string, blockHash blockchain.Hash32) (*blockchain.Block, error) {
	hashHex := hex.EncodeToString(blockHash[:])
	url := fmt.Sprintf("http://%s/block/%s", nodeAddr, hashHex)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	var blockResp node.BlockResponse
	if err := json.NewDecoder(resp.Body).Decode(&blockResp); err != nil {
		return nil, fmt.Errorf("failed to decode block response: %w", err)
	}

	if !blockResp.Found {
		return nil, fmt.Errorf("block not found")
	}

	blockData, err := json.Marshal(blockResp.Block)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block data: %w", err)
	}

	var block blockchain.Block
	if err := json.Unmarshal(blockData, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

func getContractInfo(t *testing.T, nodeAddr string, contractAddr blockchain.PublicKey) *node.ContractResponse {
	addrHex := hex.EncodeToString(contractAddr[:])
	url := fmt.Sprintf("http://%s/contract/%s", nodeAddr, addrHex)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to get contract info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP error: %s", resp.Status)
	}

	var contractResp node.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&contractResp); err != nil {
		t.Fatalf("Failed to decode contract response: %v", err)
	}

	return &contractResp
}