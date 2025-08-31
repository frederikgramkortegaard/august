package store

import (
	"testing"

	"gocuria/blockchain"
)

func TestMemoryChainStore(t *testing.T) {
	store := NewMemoryChainStore()

	// Test initial state
	t.Run("initial state", func(t *testing.T) {
		height, err := store.GetChainHeight()
		if err != nil {
			t.Fatalf("GetChainHeight() failed: %v", err)
		}
		if height != 0 {
			t.Errorf("Expected initial height 0, got %d", height)
		}

		head, err := store.GetHeadBlock()
		// Empty chain might return nil block but no error, that's ok
		if head != nil && err == nil {
			t.Error("Expected nil head block or error from empty chain")
		}
	})

	// Test adding genesis block
	t.Run("add genesis block", func(t *testing.T) {
		err := store.AddBlock(blockchain.GenesisBlock)
		if err != nil {
			t.Fatalf("AddBlock(GenesisBlock) failed: %v", err)
		}

		height, err := store.GetChainHeight()
		if err != nil {
			t.Fatalf("GetChainHeight() failed: %v", err)
		}
		if height != 1 {
			t.Errorf("Expected height 1 after adding genesis, got %d", height)
		}

		head, err := store.GetHeadBlock()
		if err != nil {
			t.Fatalf("GetHeadBlock() failed: %v", err)
		}
		if head == nil {
			t.Error("GetHeadBlock() returned nil block")
		}
	})

	// Test account states
	t.Run("account states", func(t *testing.T) {
		accountStates, err := store.GetAccountStates()
		if err != nil {
			t.Fatalf("GetAccountStates() failed: %v", err)
		}

		// Should have FirstUser account from genesis
		firstUserState, exists := accountStates[blockchain.FirstUser]
		if !exists {
			t.Error("Expected FirstUser account to exist after genesis")
		}
		if firstUserState.Balance != 10000000 {
			t.Errorf("Expected FirstUser balance 10000000, got %d", firstUserState.Balance)
		}

		// Test GetAccountState for existing account
		state, err := store.GetAccountState(blockchain.FirstUser)
		if err != nil {
			t.Fatalf("GetAccountState(FirstUser) failed: %v", err)
		}
		if state.Balance != 10000000 {
			t.Errorf("Expected balance 10000000, got %d", state.Balance)
		}

		// Test GetAccountState for non-existing account
		nonExistentKey := blockchain.PublicKey{0xFF}
		_, err = store.GetAccountState(nonExistentKey)
		if err == nil {
			t.Error("Expected error when getting non-existent account state")
		}
	})

	// Test GetBlockByHash
	t.Run("get block by hash", func(t *testing.T) {
		genesisHash := blockchain.HashBlockHeader(&blockchain.GenesisBlock.Header)

		block, err := store.GetBlockByHash(genesisHash)
		if err != nil {
			t.Fatalf("GetBlockByHash() failed: %v", err)
		}
		if block == nil {
			t.Error("GetBlockByHash() returned nil block")
		}

		// Test with non-existent hash
		nonExistentHash := blockchain.Hash32{0xFF, 0xFF, 0xFF}
		block, err = store.GetBlockByHash(nonExistentHash)
		// Should return nil block for non-existent hash (that's ok)
		if block != nil {
			t.Error("Expected nil block for non-existent hash")
		}
	})

	// Test GetChain
	t.Run("get chain", func(t *testing.T) {
		chain, err := store.GetChain()
		if err != nil {
			t.Fatalf("GetChain() failed: %v", err)
		}
		if chain == nil {
			t.Error("GetChain() returned nil")
		}
		if len(chain.Blocks) != 1 {
			t.Errorf("Expected 1 block in chain, got %d", len(chain.Blocks))
		}
		if len(chain.AccountStates) != 1 {
			t.Errorf("Expected 1 account state, got %d", len(chain.AccountStates))
		}
	})
}

func TestMemoryChainStoreValidation(t *testing.T) {
	store := NewMemoryChainStore()
	store.AddBlock(blockchain.GenesisBlock)

	// Test adding invalid block (this should fail validation in store.AddBlock)
	invalidBlock := &blockchain.Block{
		Header: blockchain.BlockHeader{
			Version:      1,
			PreviousHash: blockchain.Hash32{0xFF}, // Wrong previous hash
			Timestamp:    1000,
			Nonce:        0,
			MerkleRoot:   blockchain.Hash32{},
		},
		Transactions: []blockchain.Transaction{},
	}

	err := store.AddBlock(invalidBlock)
	if err == nil {
		t.Error("Expected error when adding invalid block")
	}

	// Chain should still have only genesis block
	height, _ := store.GetChainHeight()
	if height != 1 {
		t.Errorf("Expected height to remain 1 after failed block addition, got %d", height)
	}
}
