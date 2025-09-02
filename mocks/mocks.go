package mocks

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"gocuria/blockchain"
	"time"
)

// GenerateKeyPair generates a new ed25519 key pair
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return pub, priv
}

// GenerateValidTransaction creates a valid transaction from sender to receiver
// If amount is -1, generates a random valid amount (between 1 and sender's balance)
func GenerateValidTransaction(chain *blockchain.Chain, senderPub ed25519.PublicKey, senderPriv ed25519.PrivateKey, receiverPub ed25519.PublicKey, amount int64) (*blockchain.Transaction, error) {
	// Convert ed25519 keys to blockchain format
	var fromKey blockchain.PublicKey
	copy(fromKey[:], senderPub)
	
	var toKey blockchain.PublicKey
	copy(toKey[:], receiverPub)
	
	// Get sender's current nonce
	senderState, exists := chain.AccountStates[fromKey]
	if !exists {
		return nil, errors.New("insufficient balance")
	}
	
	// Handle random amount generation
	var finalAmount uint64
	if amount == -1 {
		if senderState.Balance == 0 {
			return nil, errors.New("insufficient balance")
		}
		// Generate random amount between 1 and balance
		maxAmount := senderState.Balance
		if maxAmount > 1 {
			// Use crypto/rand for secure randomness
			randomBytes := make([]byte, 8)
			rand.Read(randomBytes)
			// Convert to uint64 and modulo by balance
			randomValue := uint64(randomBytes[0])<<56 | uint64(randomBytes[1])<<48 | 
				uint64(randomBytes[2])<<40 | uint64(randomBytes[3])<<32 |
				uint64(randomBytes[4])<<24 | uint64(randomBytes[5])<<16 | 
				uint64(randomBytes[6])<<8 | uint64(randomBytes[7])
			finalAmount = (randomValue % maxAmount) + 1
		} else {
			finalAmount = 1
		}
	} else if amount < 0 {
		return nil, errors.New("invalid amount: must be positive or -1 for random")
	} else {
		finalAmount = uint64(amount)
	}
	
	// Check balance
	if senderState.Balance < finalAmount {
		return nil, errors.New("insufficient balance")
	}
	
	// Create transaction
	tx := &blockchain.Transaction{
		From:   fromKey,
		To:     toKey,
		Amount: finalAmount,
		Nonce:  senderState.Nonce + 1,
	}
	
	// Sign it
	blockchain.SignTransaction(tx, senderPriv)
	
	return tx, nil
}

// GenerateValidTransactionWithNonce creates a valid transaction with a specified nonce
// If amount is -1, generates a random valid amount (between 1 and sender's balance)
func GenerateValidTransactionWithNonce(chain *blockchain.Chain, senderPub ed25519.PublicKey, senderPriv ed25519.PrivateKey, receiverPub ed25519.PublicKey, amount int64, nonce uint64) (*blockchain.Transaction, error) {
	// Convert ed25519 keys to blockchain format
	var fromKey blockchain.PublicKey
	copy(fromKey[:], senderPub)
	
	var toKey blockchain.PublicKey
	copy(toKey[:], receiverPub)
	
	// Get sender's current state
	senderState, exists := chain.AccountStates[fromKey]
	if !exists {
		return nil, errors.New("insufficient balance")
	}
	
	// Handle random amount generation
	var finalAmount uint64
	if amount == -1 {
		if senderState.Balance == 0 {
			return nil, errors.New("insufficient balance")
		}
		// Generate random amount between 1 and balance
		maxAmount := senderState.Balance
		if maxAmount > 1 {
			// Use crypto/rand for secure randomness
			randomBytes := make([]byte, 8)
			rand.Read(randomBytes)
			// Convert to uint64 and modulo by balance
			randomValue := uint64(randomBytes[0])<<56 | uint64(randomBytes[1])<<48 | 
				uint64(randomBytes[2])<<40 | uint64(randomBytes[3])<<32 |
				uint64(randomBytes[4])<<24 | uint64(randomBytes[5])<<16 | 
				uint64(randomBytes[6])<<8 | uint64(randomBytes[7])
			finalAmount = (randomValue % maxAmount) + 1
		} else {
			finalAmount = 1
		}
	} else if amount < 0 {
		return nil, errors.New("invalid amount: must be positive or -1 for random")
	} else {
		finalAmount = uint64(amount)
	}
	
	// Check balance
	if senderState.Balance < finalAmount {
		return nil, errors.New("insufficient balance")
	}
	
	// Create transaction with specified nonce
	tx := &blockchain.Transaction{
		From:   fromKey,
		To:     toKey,
		Amount: finalAmount,
		Nonce:  nonce,
	}
	
	// Sign it
	blockchain.SignTransaction(tx, senderPriv)
	
	return tx, nil
}

// GenerateValidMinedBlock creates a new valid mined block with coinbase
func GenerateValidMinedBlock(chain *blockchain.Chain, minerPub ed25519.PublicKey, transactions []blockchain.Transaction) (*blockchain.Block, error) {
	// Convert miner key
	var minerKey blockchain.PublicKey
	copy(minerKey[:], minerPub)
	
	// Get latest block
	latestBlock := chain.Blocks[len(chain.Blocks)-1]
	previousHash := blockchain.HashBlockHeader(&latestBlock.Header)
	
	// Create coinbase transaction (miner reward)
	coinbase := blockchain.Transaction{
		From:   blockchain.PublicKey{}, // Empty for coinbase
		To:     minerKey,
		Amount: 50, // Standard reward
		Nonce:  0,
	}
	
	// Create block parameters
	blockParams := blockchain.BlockCreationParams{
		Version:      1,
		PreviousHash: previousHash,
		Height:       uint64(len(chain.Blocks)),
		PreviousWork: latestBlock.Header.TotalWork,
		Coinbase:     coinbase,
		Transactions: transactions,
		Timestamp:    uint64(time.Now().Unix()),
		Difficulty:   blockchain.GetTargetDifficulty(len(chain.Blocks), chain.Blocks),
	}
	
	// Mine the block (this will find valid nonce)
	newBlock, err := blockchain.NewBlock(blockParams)
	if err != nil {
		return nil, err
	}
	
	return &newBlock, nil
}

// GenerateInvalidBlock creates a block with invalid properties (for testing rejection)
func GenerateInvalidBlock(chain *blockchain.Chain) *blockchain.Block {
	// Create a block with wrong previous hash
	return &blockchain.Block{
		Header: blockchain.BlockHeader{
			Version:      1,
			PreviousHash: blockchain.Hash32{}, // Invalid - should be hash of previous block
			Height:       uint64(len(chain.Blocks)),
			Timestamp:    uint64(time.Now().Unix()),
			MerkleRoot:   blockchain.Hash32{},
			TotalWork:    "0", // Invalid work
			Nonce:        0,
		},
		Transactions: []blockchain.Transaction{},
	}
}

// ApplyBlockToChain validates and applies a block to the chain (helper)
func ApplyBlockToChain(chain *blockchain.Chain, block *blockchain.Block) error {
	return blockchain.ValidateAndApplyBlock(block, chain)
}