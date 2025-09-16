package storage

import (
	"august/blockchain"
	"august/config"
	"august/utils"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
)

func CreateOrLoadDatabase(dbname string, opts *pebble.Options) (*pebble.DB, error) {
	db, err := pebble.Open(dbname, opts)
	if err != nil {
		log.Fatal(err)
	}
	return db, nil
}

// StoreBlock stores a complete block as JSON by its hash
func StoreBlock(db *pebble.DB, block *blockchain.Block) error {
	// Store the block by hash
	jsonData, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	hash := block.Header.GetHash()
	blockKey := append([]byte("block:"), hash[:]...)

	if err := db.Set(blockKey, jsonData, pebble.NoSync); err != nil {
		return fmt.Errorf("failed to store block: %w", err)
	}

	return nil
}

// GetBlock retrieves a complete block by hash
func GetBlock(db *pebble.DB, blockHash blockchain.Hash32) (*blockchain.Block, error) {
	key := append([]byte("block:"), blockHash[:]...)

	val, closer, err := db.Get(key)
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, fmt.Errorf("block not found")
		}
		return nil, fmt.Errorf("failed to get block: %w", err)
	}
	defer closer.Close()

	var block blockchain.Block
	if err := json.Unmarshal(val, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

// GetChain reconstructs the complete chain from the stored chain manifest
func GetChain(db *pebble.DB) (*blockchain.Chain, error) {
	// Get the chain manifest (list of block hashes)
	manifestData, closer, err := db.Get([]byte("chain:manifest"))
	if err != nil {
		if err == pebble.ErrNotFound {
			// No chain stored yet, return empty chain
			return &blockchain.Chain{
				Blocks:        make([]*blockchain.Block, 0),
				AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
			}, nil
		}
		return nil, fmt.Errorf("failed to get chain manifest: %w", err)
	}
	defer closer.Close()

	// Parse the manifest (JSON array of hashes)
	var blockHashes []blockchain.Hash32
	if err := json.Unmarshal(manifestData, &blockHashes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chain manifest: %w", err)
	}

	// Reconstruct the chain by fetching each block
	chain := &blockchain.Chain{
		Blocks:        make([]*blockchain.Block, 0, len(blockHashes)),
		AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
	}

	for i, hash := range blockHashes {
		block, err := GetBlock(db, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to get block %d: %w", i, err)
		}
		chain.Blocks = append(chain.Blocks, block)

		// Rebuild account states by applying all transactions
		for _, tx := range block.Transactions {
			// Coinbase transaction
			if tx.From == (blockchain.PublicKey{}) {
				if toState, ok := chain.AccountStates[tx.To]; ok {
					newBalance, err := utils.SafeAdd(toState.Balance, tx.Amount)
					if err != nil {
						return nil, fmt.Errorf("coinbase balance overflow: %w", err)
					}
					toState.Balance = newBalance
				} else {
					chain.AccountStates[tx.To] = &blockchain.AccountState{
						Address: tx.To,
						Balance: tx.Amount,
						Nonce:   0,
					}
				}
			} else {
				// Regular transaction
				fromState := chain.AccountStates[tx.From]
				if fromState != nil {
					// Calculate estimated gas cost for this transaction
					estimatedGas := config.GasTransfer
					if tx.To == (blockchain.PublicKey{}) && len(tx.Instructions) > 0 {
						estimatedGas = config.GasContractDeploy
					}
					estimatedGasCost := estimatedGas * tx.GasPrice

					total, err := utils.SafeAdd(tx.Amount, estimatedGasCost)
					if err != nil {
						return nil, fmt.Errorf("transaction total overflow: %w", err)
					}
					newBalance, err := utils.SafeSubtract(fromState.Balance, total)
					if err != nil {
						return nil, fmt.Errorf("sender balance underflow: %w", err)
					}
					fromState.Balance = newBalance
					fromState.Nonce += 1
				}

				if toState, ok := chain.AccountStates[tx.To]; ok {
					newBalance, err := utils.SafeAdd(toState.Balance, tx.Amount)
					if err != nil {
						return nil, fmt.Errorf("recipient balance overflow: %w", err)
					}
					toState.Balance = newBalance
				} else {
					chain.AccountStates[tx.To] = &blockchain.AccountState{
						Address: tx.To,
						Balance: tx.Amount,
						Nonce:   0,
					}
				}
			}
		}
	}

	// Initialize tip and block index
	chain.InitializeIndexes()

	return chain, nil
}

// StoreChainManifest stores the list of block hashes that make up the current chain
func StoreChainManifest(db *pebble.DB, chain *blockchain.Chain) error {
	// Build list of block hashes
	blockHashes := make([]blockchain.Hash32, len(chain.Blocks))
	for i, block := range chain.Blocks {
		blockHashes[i] = block.Header.GetHash()
	}

	// Marshal to JSON
	manifestData, err := json.Marshal(blockHashes)
	if err != nil {
		return fmt.Errorf("failed to marshal chain manifest: %w", err)
	}

	// Store it
	if err := db.Set([]byte("chain:manifest"), manifestData, pebble.NoSync); err != nil {
		return fmt.Errorf("failed to store chain manifest: %w", err)
	}

	return nil
}

// Helper functions
func uint64ToBytes(n uint64) []byte {
	bytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		bytes[7-i] = byte(n >> (8 * i))
	}
	return bytes
}

func getCurrentHeight(tipData []byte) uint64 {
	if len(tipData) < 40 { // 32 bytes hash + 8 bytes height
		return 0
	}
	heightBytes := tipData[32:]
	var height uint64
	for i := 0; i < 8; i++ {
		height |= uint64(heightBytes[7-i]) << (8 * i)
	}
	return height
}

// StoreChainTip stores just the tip reference for quick access
func StoreChainTip(db *pebble.DB, tipHash blockchain.Hash32, height uint64) error {
	tipKey := []byte("chain:tip")
	heightBytes := uint64ToBytes(height)
	tipData := append(tipHash[:], heightBytes...)

	if err := db.Set(tipKey, tipData, pebble.NoSync); err != nil {
		return fmt.Errorf("failed to store chain tip: %w", err)
	}
	return nil
}

// GetChainTip retrieves just the tip block without reconstructing the whole chain
func GetChainTip(db *pebble.DB) (*blockchain.Block, error) {
	tipKey := []byte("chain:tip")
	tipData, closer, err := db.Get(tipKey)
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, fmt.Errorf("no chain tip found")
		}
		return nil, fmt.Errorf("failed to get chain tip: %w", err)
	}
	defer closer.Close()

	var tipHash blockchain.Hash32
	copy(tipHash[:], tipData[:32])

	return GetBlock(db, tipHash)
}
