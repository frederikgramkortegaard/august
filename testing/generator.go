package testing

import (
	"crypto/ed25519"
	"crypto/rand"
	"gocuria/blockchain"
	mathrand "math/rand"
	"time"
)

// TestAccount holds a complete key pair for testing
type TestAccount struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// GetFirstUserTestAccount creates a TestAccount for the FirstUser (for testing only)
// In production, this private key would not be accessible
func GetFirstUserTestAccount() TestAccount {
	// Fixed seed for deterministic FirstUser keypair (testing only!)
	seed := [32]byte{
		0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0,
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
		0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11,
		0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99,
	}

	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return TestAccount{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}
}

// GenerateTestAccounts creates N test accounts with keypairs
func GenerateTestAccounts(count int) []TestAccount {
	accounts := make([]TestAccount, count)

	for i := 0; i < count; i++ {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			panic("Failed to generate keypair: " + err.Error())
		}

		accounts[i] = TestAccount{
			PrivateKey: priv,
			PublicKey:  pub,
		}
	}

	return accounts
}

// PublicKeyToArray converts ed25519.PublicKey to blockchain.PublicKey
func PublicKeyToArray(pub ed25519.PublicKey) blockchain.PublicKey {
	var pubKey blockchain.PublicKey
	copy(pubKey[:], pub)
	return pubKey
}

// GenerateValidTransaction creates a properly signed transaction
func GenerateValidTransaction(from TestAccount, to blockchain.PublicKey, amount, nonce uint64) blockchain.Transaction {
	tx := blockchain.Transaction{
		From:      PublicKeyToArray(from.PublicKey),
		To:        to,
		Amount:    amount,
		Nonce:     nonce,
		Signature: blockchain.Signature{}, // Will be filled by signing
	}

	// Sign the transaction
	blockchain.SignTransaction(&tx, from.PrivateKey)

	return tx
}

// GenerateCoinbaseTransaction creates a coinbase transaction (mining reward)
func GenerateCoinbaseTransaction(to blockchain.PublicKey, amount uint64) blockchain.Transaction {
	return blockchain.Transaction{
		From:      blockchain.PublicKey{}, // Zero/empty for coinbase
		To:        to,
		Amount:    amount,
		Nonce:     0,                  // Coinbase doesn't need nonce
		Signature: blockchain.Signature{}, // Coinbase doesn't need signature
	}
}

// GenerateRandomTransactions creates random valid transactions with proper nonces
func GenerateRandomTransactions(accounts []TestAccount, count int) []blockchain.Transaction {
	if len(accounts) < 2 {
		panic("Need at least 2 accounts to generate transactions")
	}

	r := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	transactions := make([]blockchain.Transaction, 0, count)

	// Track nonces for each account during generation
	nonces := make(map[blockchain.PublicKey]uint64)
	for _, account := range accounts {
		nonces[PublicKeyToArray(account.PublicKey)] = 0 // Start at 0 (will increment to 1 for first tx)
	}

	for i := 0; i < count; i++ {
		fromIdx := r.Intn(len(accounts))
		toIdx := r.Intn(len(accounts))

		// Make sure from != to
		for toIdx == fromIdx {
			toIdx = r.Intn(len(accounts))
		}

		fromPubKey := PublicKeyToArray(accounts[fromIdx].PublicKey)
		
		// Use sequential nonce for this account
		nonces[fromPubKey]++
		nonce := nonces[fromPubKey]

		// Random amount between 1-100 (keep small to avoid balance issues)
		amount := uint64(r.Intn(100) + 1)

		transactions = append(transactions, GenerateValidTransaction(
			accounts[fromIdx],
			PublicKeyToArray(accounts[toIdx].PublicKey),
			amount,
			nonce,
		))
	}

	return transactions
}

// GenerateTestBlock creates a block with given transactions (NOT mined)
func GenerateTestBlock(prevHash [32]byte, transactions []blockchain.Transaction, timestamp uint64) blockchain.Block {
	if timestamp == 0 {
		timestamp = uint64(time.Now().Unix())
	}

	merkleRoot := blockchain.MerkleTransactions(transactions)

	header := blockchain.BlockHeader{
		Version:      1,
		PreviousHash: prevHash,
		Timestamp:    timestamp,
		Nonce:        0, // Not mined
		MerkleRoot:   merkleRoot,
	}

	return blockchain.Block{
		Header:       header,
		Transactions: transactions,
	}
}

// GenerateMinedTestBlock creates a properly mined block (like from network)
func GenerateMinedTestBlock(prevHash [32]byte, transactions []blockchain.Transaction, timestamp uint64) blockchain.Block {
	block := GenerateTestBlock(prevHash, transactions, timestamp)

	// Mine the block to find valid nonce
	nonce, err := blockchain.MineCorrectNonce(&block.Header)
	if err != nil {
		panic("Failed to mine test block: " + err.Error())
	}
	block.Header.Nonce = nonce

	return block
}

// GeneratePrebuiltChain creates a simple test chain (like downloaded from network)
func GeneratePrebuiltChain(blockCount int, accountCount int, transactionsPerBlock int) []*blockchain.Block {
	accounts := GenerateTestAccounts(accountCount)
	firstUser := GetFirstUserTestAccount()
	blocks := make([]*blockchain.Block, 0, blockCount+2) // +2 for genesis and distribution block

	// Global nonce tracking across all blocks
	globalNonces := make(map[blockchain.PublicKey]uint64)
	
	// Initialize nonces for all accounts (start at 0, will increment for first transaction)
	for _, account := range accounts {
		globalNonces[PublicKeyToArray(account.PublicKey)] = 0
	}
	globalNonces[blockchain.FirstUser] = 0 // FirstUser starts at 0 too

	// Start with genesis block
	blocks = append(blocks, blockchain.GenesisBlock)
	var prevHash [32]byte
	prevHash = blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header)

	// Block 1: Distribution block - FirstUser distributes coins to test accounts
	distributionTxs := make([]blockchain.Transaction, 0, accountCount+1)
	
	// Coinbase for miner
	miner := PublicKeyToArray(accounts[0].PublicKey)
	coinbase := GenerateCoinbaseTransaction(miner, 5000)
	distributionTxs = append(distributionTxs, coinbase)

	// FirstUser distributes 10,000 coins to each test account
	for _, account := range accounts {
		// Increment FirstUser's nonce for each distribution transaction
		globalNonces[blockchain.FirstUser]++
		
		distributionTx := GenerateValidTransaction(
			firstUser,
			PublicKeyToArray(account.PublicKey),
			10000, // Give each account 10k coins
			globalNonces[blockchain.FirstUser],
		)
		distributionTxs = append(distributionTxs, distributionTx)
	}

	// Create distribution block
	timestamp := uint64(time.Now().Unix()) + 600 // 10 min after genesis
	distributionBlock := GenerateMinedTestBlock(prevHash, distributionTxs, timestamp)
	blocks = append(blocks, &distributionBlock)
	prevHash = blockchain.HashBlockHeader(&distributionBlock.Header)

	// Generate remaining blocks with random transactions between funded accounts
	r := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	
	for i := 0; i < blockCount; i++ {
		allTxs := make([]blockchain.Transaction, 0, transactionsPerBlock)
		
		// First transaction is always coinbase
		minerIdx := (i + 1) % len(accounts)
		coinbase := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[minerIdx].PublicKey), 5000)
		allTxs = append(allTxs, coinbase)

		// Generate regular transactions with proper global nonce tracking
		for j := 1; j < transactionsPerBlock; j++ {
			fromIdx := r.Intn(len(accounts))
			toIdx := r.Intn(len(accounts))

			// Make sure from != to
			for toIdx == fromIdx {
				toIdx = r.Intn(len(accounts))
			}

			fromPubKey := PublicKeyToArray(accounts[fromIdx].PublicKey)
			
			// Use and increment the global nonce for this account
			globalNonces[fromPubKey]++
			nonce := globalNonces[fromPubKey]

			// Random amount between 1-100 (keep small to avoid balance issues)
			amount := uint64(r.Intn(100) + 1)

			tx := GenerateValidTransaction(
				accounts[fromIdx],
				PublicKeyToArray(accounts[toIdx].PublicKey),
				amount,
				nonce,
			)
			allTxs = append(allTxs, tx)
		}

		// Create mined block
		timestamp := uint64(time.Now().Unix()) + uint64((i+2)*600) // Continue timestamp sequence
		block := GenerateMinedTestBlock(prevHash, allTxs, timestamp)

		blocks = append(blocks, &block)
		prevHash = blockchain.HashBlockHeader(&block.Header)
	}

	return blocks
}

// GenerateInvalidTransactions creates transactions with common issues
func GenerateInvalidTransactions(accounts []TestAccount) map[string]blockchain.Transaction {
	invalid := make(map[string]blockchain.Transaction)

	if len(accounts) >= 2 {
		// Invalid signature (tampered amount)
		validTx := GenerateValidTransaction(accounts[0], PublicKeyToArray(accounts[1].PublicKey), 100, 1)
		validTx.Amount = 999 // Tamper with amount after signing
		invalid["tampered_amount"] = validTx

		// Zero amount
		invalid["zero_amount"] = GenerateValidTransaction(accounts[0], PublicKeyToArray(accounts[1].PublicKey), 0, 1)

		// Self-transaction
		invalid["self_transaction"] = GenerateValidTransaction(accounts[0], PublicKeyToArray(accounts[0].PublicKey), 100, 1)
	}

	return invalid
}

// GetSimpleTransferChain - For testing basic transfers
func GetSimpleTransferChain() []*blockchain.Block {
	accounts := GenerateTestAccounts(3)

	// Block 1: Initial distribution
	coinbase1 := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[0].PublicKey), 1000)
	block1 := GenerateMinedTestBlock([32]byte{}, []blockchain.Transaction{coinbase1}, 1000)

	// Block 2: Transfer from account 0 to account 1
	transfer1 := GenerateValidTransaction(accounts[0], PublicKeyToArray(accounts[1].PublicKey), 300, 1)
	coinbase2 := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[1].PublicKey), 1000)
	block2 := GenerateMinedTestBlock(blockchain.HashBlockHeader(&block1.Header), []blockchain.Transaction{coinbase2, transfer1}, 2000)

	return []*blockchain.Block{&block1, &block2}
}

// GetHighVolumeChain - For stress testing
func GetHighVolumeChain() []*blockchain.Block {
	return GeneratePrebuiltChain(100, 50, 20) // 100 blocks, 50 accounts, 20 txs each
}

// GetCircularTransferChain - For testing A->B->C->A pattern
func GetCircularTransferChain() []*blockchain.Block {
	accounts := GenerateTestAccounts(3)
	blocks := make([]*blockchain.Block, 0, 4)

	var prevHash [32]byte

	// Block 1: Give everyone initial coins via coinbase
	coinbase1 := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[0].PublicKey), 1000)
	coinbase2 := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[1].PublicKey), 1000)
	coinbase3 := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[2].PublicKey), 1000)
	block1 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase1, coinbase2, coinbase3}, 1000)
	blocks = append(blocks, &block1)
	prevHash = blockchain.HashBlockHeader(&block1.Header)

	// Block 2: A -> B
	transfer1 := GenerateValidTransaction(accounts[0], PublicKeyToArray(accounts[1].PublicKey), 100, 1)
	coinbase := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[0].PublicKey), 1000)
	block2 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase, transfer1}, 2000)
	blocks = append(blocks, &block2)
	prevHash = blockchain.HashBlockHeader(&block2.Header)

	// Block 3: B -> C
	transfer2 := GenerateValidTransaction(accounts[1], PublicKeyToArray(accounts[2].PublicKey), 100, 1)
	coinbase = GenerateCoinbaseTransaction(PublicKeyToArray(accounts[1].PublicKey), 1000)
	block3 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase, transfer2}, 3000)
	blocks = append(blocks, &block3)
	prevHash = blockchain.HashBlockHeader(&block3.Header)

	// Block 4: C -> A (completing the circle)
	transfer3 := GenerateValidTransaction(accounts[2], PublicKeyToArray(accounts[0].PublicKey), 100, 1)
	coinbase = GenerateCoinbaseTransaction(PublicKeyToArray(accounts[2].PublicKey), 1000)
	block4 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase, transfer3}, 4000)
	blocks = append(blocks, &block4)

	return blocks
}

// GetDoubleSpendChain - For testing double-spend detection
func GetDoubleSpendChain() []*blockchain.Block {
	accounts := GenerateTestAccounts(3)
	blocks := make([]*blockchain.Block, 0, 3)

	var prevHash [32]byte

	// Block 1: Give account[0] initial coins
	coinbase1 := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[0].PublicKey), 1000)
	block1 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase1}, 1000)
	blocks = append(blocks, &block1)
	prevHash = blockchain.HashBlockHeader(&block1.Header)

	// Block 2: Legitimate transaction
	transfer1 := GenerateValidTransaction(accounts[0], PublicKeyToArray(accounts[1].PublicKey), 500, 1)
	coinbase := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[1].PublicKey), 1000)
	block2 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase, transfer1}, 2000)
	blocks = append(blocks, &block2)
	prevHash = blockchain.HashBlockHeader(&block2.Header)

	// Block 3: Double spend attempt (same nonce, different recipient)
	transfer2 := GenerateValidTransaction(accounts[0], PublicKeyToArray(accounts[2].PublicKey), 500, 1) // Same nonce!
	coinbase = GenerateCoinbaseTransaction(PublicKeyToArray(accounts[2].PublicKey), 1000)
	block3 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase, transfer2}, 3000)
	blocks = append(blocks, &block3)

	return blocks
}

// GetInvalidNonceChain - For testing nonce validation
func GetInvalidNonceChain() []*blockchain.Block {
	accounts := GenerateTestAccounts(2)
	blocks := make([]*blockchain.Block, 0, 3)

	var prevHash [32]byte

	// Block 1: Give account[0] initial coins
	coinbase1 := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[0].PublicKey), 1000)
	block1 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase1}, 1000)
	blocks = append(blocks, &block1)
	prevHash = blockchain.HashBlockHeader(&block1.Header)

	// Block 2: Valid transaction (nonce 1)
	transfer1 := GenerateValidTransaction(accounts[0], PublicKeyToArray(accounts[1].PublicKey), 100, 1)
	coinbase := GenerateCoinbaseTransaction(PublicKeyToArray(accounts[1].PublicKey), 1000)
	block2 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase, transfer1}, 2000)
	blocks = append(blocks, &block2)
	prevHash = blockchain.HashBlockHeader(&block2.Header)

	// Block 3: Invalid nonce (should be 2, but using 5)
	transfer2 := GenerateValidTransaction(accounts[0], PublicKeyToArray(accounts[1].PublicKey), 100, 5) // Wrong nonce!
	coinbase = GenerateCoinbaseTransaction(PublicKeyToArray(accounts[1].PublicKey), 1000)
	block3 := GenerateMinedTestBlock(prevHash, []blockchain.Transaction{coinbase, transfer2}, 3000)
	blocks = append(blocks, &block3)

	return blocks
}
