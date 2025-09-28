package testutils

import (
	"august/blockchain"
	"august/config"
	"august/utils"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"time"
)

// CreateTestKeyPair generates a new ed25519 key pair for testing
func CreateTestKeyPair() (blockchain.PublicKey, ed25519.PrivateKey) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic("Failed to generate test key pair: " + err.Error())
	}
	var publicKey blockchain.PublicKey
	copy(publicKey[:], pubKey)
	return publicKey, privKey
}

// CreateCoinbaseTransaction creates a coinbase transaction for block rewards
func CreateCoinbaseTransaction(beneficiary blockchain.PublicKey, blockHeight uint64) *blockchain.Transaction {
	// Coinbase transactions have zero hash as From address
	return &blockchain.Transaction{
		From:             blockchain.PublicKey{}, // Zero hash for coinbase
		To:               beneficiary,
		Amount:           config.BlockReward, // 50 AUG block reward
		Nonce:            0,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          config.MainnetChainID,
		Instructions:     []blockchain.Instruction{},
		GasLimit:         0, // Coinbase doesn't consume gas
		GasPrice:         0,
		Data:             []byte{},
		Signature:        blockchain.Signature{}, // Coinbase doesn't need signature
	}
}

// CreateTestTransaction creates a valid but unsigned transaction
func CreateTestTransaction(from, to blockchain.PublicKey, amount uint64, nonce uint64) *blockchain.Transaction {
	return &blockchain.Transaction{
		From:             from,
		To:               to,
		Amount:           amount,
		Nonce:            nonce,
		Timestamp:        uint64(time.Now().Unix()),
		ChainID:          config.TestnetChainID,
		Instructions:     []blockchain.Instruction{},
		GasLimit:         config.GasTransfer,
		GasPrice:         1,
		Data:             []byte{},
		Signature:        blockchain.Signature{}, // Will be set by SignTransaction
	}
}

// SignTransaction signs a transaction with the given private key
func SignTransaction(tx *blockchain.Transaction, privKey ed25519.PrivateKey) {
	txHash := tx.GetHash()
	signature := ed25519.Sign(privKey, txHash[:])
	copy(tx.Signature[:], signature)
}

// calculateGasUsed calculates total gas used by transactions
func calculateGasUsed(txs []*blockchain.Transaction) uint64 {
	var total uint64
	for _, tx := range txs {
		// Coinbase transactions (From == zero) don't use gas
		if tx.From == (blockchain.PublicKey{}) {
			continue
		}
		total += config.GasTransfer                           // Base gas for each transaction
		total += uint64(len(tx.Data)) * config.GasDataPerByte // Gas for data
	}
	return total
}

// calculateStateRoot creates a simple state root hash for testing
// In production this would be a proper merkle patricia trie root
func calculateStateRoot(accountStates map[blockchain.PublicKey]*blockchain.AccountState) blockchain.Hash32 {
	hasher := sha256.New()

	// Sort addresses for deterministic hash
	for pubKey, state := range accountStates {
		hasher.Write(pubKey[:])
		hasher.Write(utils.Uint64ToBytes(state.Balance))
		hasher.Write(utils.Uint64ToBytes(state.Nonce))
	}

	var root blockchain.Hash32
	copy(root[:], hasher.Sum(nil))
	return root
}

// mineHeader performs proof-of-work to make header valid
func mineHeader(header *blockchain.BlockHeader) {
	fmt.Printf("DEBUG: Mining header with target bits: 0x%x\n", header.Bits)

	for header.Nonce < 1000000 { // Prevent infinite loop
		hash := header.GetHash()
		if blockchain.BlockHashMeetsDifficulty(hash, header.Bits) {
			fmt.Printf("DEBUG: Mined successfully! Nonce: %d, Hash: %x\n", header.Nonce, hash[:8])
			return // Found valid nonce
		}
		header.Nonce++
	}
	panic("Could not mine header within nonce limit - target may be too difficult")
}

// CreateTestHeader creates a VALID header that will pass validation
func CreateTestHeader(parent *blockchain.BlockHeader, txs []*blockchain.Transaction) *blockchain.BlockHeader {
	var parentHash blockchain.Hash32
	var height uint64 = 0
	var parentWork *big.Int = big.NewInt(0)

	if parent != nil {
		parentHash = parent.GetHash()
		height = parent.Height + 1
		parentWork, _ = new(big.Int).SetString(parent.TotalWork, 10)
	}

	targetBits := config.GetTargetCompact()
	fmt.Printf("DEBUG: Creating test header with target bits: 0x%x\n", targetBits)

	// Ensure timestamps are in ascending order
	timestamp := uint64(time.Now().Unix())
	if parent != nil && timestamp <= parent.Timestamp {
		timestamp = parent.Timestamp + 1
	}

	header := &blockchain.BlockHeader{
		Version:      1,
		PreviousHash: parentHash,
		Height:       height,
		Timestamp:    timestamp,
		Nonce:        0,                                              // Will be set by mining
		MerkleRoot:   blockchain.MerkleTransactions(txsToValue(txs)), // ACTUAL merkle root
		StateRoot:    blockchain.Hash32{},                            // TODO: Calculate from account states in tests
		ReceiptRoot:  blockchain.Hash32{},                            // Empty for simple tests
		Bits:         targetBits,
		TotalWork:    "0", // Will be calculated after mining
		GasLimit:     1000000,
		GasUsed:      calculateGasUsed(txs),
		Beneficiary:  config.FirstUser,
	}

	// Calculate total work BEFORE mining (so hash doesn't change)
	blockWork := blockchain.CalculateBlockWorkFromBits(header.Bits)
	blockWorkInt, _ := new(big.Int).SetString(blockWork, 10)
	newTotalWork := new(big.Int).Add(parentWork, blockWorkInt)
	header.TotalWork = newTotalWork.String()

	// Mine the header to meet proof-of-work requirement
	mineHeader(header)

	return header
}

// txsToValue converts slice of pointers to slice of values for MerkleTransactions
func txsToValue(txs []*blockchain.Transaction) []blockchain.Transaction {
	result := make([]blockchain.Transaction, len(txs))
	for i, tx := range txs {
		result[i] = *tx
	}
	return result
}

// CreateTestBlock creates a block that will pass full validation
func CreateTestBlock(parent *blockchain.BlockHeader, txs []*blockchain.Transaction) *blockchain.Block {
	header := CreateTestHeader(parent, txs)

	return &blockchain.Block{
		Header:       *header,
		Transactions: txsToValue(txs),
	}
}

// CreateTestChain creates a chain of N headers starting from parent (or genesis if nil)
func CreateTestChain(parent *blockchain.BlockHeader, length int) []*blockchain.BlockHeader {
	headers := make([]*blockchain.BlockHeader, length)

	currentParent := parent
	if currentParent == nil {
		// Start from genesis
		currentParent = &blockchain.GenesisBlock.Header
	}

	for i := 0; i < length; i++ {
		headers[i] = CreateTestHeader(currentParent, []*blockchain.Transaction{})
		currentParent = headers[i]
	}

	return headers
}

// CreateTestFork creates two competing chains from a common ancestor
func CreateTestFork(commonAncestor *blockchain.BlockHeader, chainALength, chainBLength int) ([]*blockchain.BlockHeader, []*blockchain.BlockHeader) {
	chainA := CreateTestChain(commonAncestor, chainALength)
	chainB := CreateTestChain(commonAncestor, chainBLength)
	return chainA, chainB
}

// CreateHigherWorkHeader creates a header with higher work than the given header
// Useful for testing reorgs
func CreateHigherWorkHeader(parent *blockchain.BlockHeader, competingHeader *blockchain.BlockHeader) *blockchain.BlockHeader {
	// Use easier target to get higher work
	header := CreateTestHeader(parent, []*blockchain.Transaction{})

	// Adjust work to be higher than competing header
	competingWork, _ := new(big.Int).SetString(competingHeader.TotalWork, 10)
	parentWork, _ := new(big.Int).SetString(parent.TotalWork, 10)

	// Make this header have more total work
	extraWork := new(big.Int).Sub(competingWork, parentWork)
	extraWork.Add(extraWork, big.NewInt(1000)) // Add extra to ensure higher work

	currentWork, _ := new(big.Int).SetString(header.TotalWork, 10)
	targetWork := new(big.Int).Add(parentWork, extraWork)

	// If current work is already sufficient, use it; otherwise adjust
	if currentWork.Cmp(targetWork) < 0 {
		header.TotalWork = targetWork.String()
	}

	return header
}
