package blockchain

import (
	"august/avm"
	"august/utils"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
)

// ChainHead represents the current head of the blockchain
type ChainHead struct {
	Height    uint64 `json:"height"`
	Hash      Hash32 `json:"hash"`
	TotalWork string `json:"total_work"`
}

const (
	Difficulty = 1
)

// FirstUser is the genesis account that receives the initial coin supply
// Generated from fixed seed in testing/generator.go (testing only!)
var FirstUser = PublicKey{
	0x99, 0x3f, 0xe6, 0xa3, 0x6d, 0x19, 0xed, 0x52,
	0x78, 0x90, 0xa7, 0x69, 0x8e, 0x94, 0x0c, 0x3c,
	0x1c, 0x62, 0x9c, 0xa0, 0x64, 0x43, 0x83, 0x78,
	0x71, 0xcb, 0x0e, 0xd1, 0x94, 0x35, 0x22, 0x9b,
}

type PublicKey [ed25519.PublicKeySize]byte // 32
type Signature [ed25519.SignatureSize]byte // 64

// Custom JSON marshaling for PublicKey
func (pk PublicKey) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(pk[:]) + `"`), nil
}

func (pk *PublicKey) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON string for PublicKey: %s", string(data))
	}

	b64String := string(data[1 : len(data)-1])
	decoded, err := base64.StdEncoding.DecodeString(b64String)
	if err != nil {
		return fmt.Errorf("failed to decode base64 '%s': %v", b64String, err)
	}

	if len(decoded) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid PublicKey size for '%s': got %d, want %d", b64String, len(decoded), ed25519.PublicKeySize)
	}

	copy(pk[:], decoded)
	return nil
}

// Custom JSON marshaling for Signature
func (sig Signature) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(sig[:]) + `"`), nil
}

func (sig *Signature) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON string for Signature")
	}

	decoded, err := base64.StdEncoding.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return fmt.Errorf("failed to decode base64: %v", err)
	}

	if len(decoded) != ed25519.SignatureSize {
		return fmt.Errorf("invalid Signature size: got %d, want %d", len(decoded), ed25519.SignatureSize)
	}

	copy(sig[:], decoded)
	return nil
}

type Transaction struct {
	From             PublicKey         `json:"from"`
	To               PublicKey         `json:"to"`
	Amount           uint64            `json:"amount"`
	Signature        Signature         `json:"signature"`
	Nonce            uint64            `json:"nonce"`
	Timestamp        uint64            `json:"timestamp"`         // Unix timestamp when transaction was created
	ChainID          uint64            `json:"chain_id"`          // Chain identifier for replay protection
	Instructions     []avm.Instruction `json:"instructions"`      // Contract runtime code (empty for regular transfers)
	InitInstructions []avm.Instruction `json:"init_instructions"` // Constructor code that runs once at deployment
	GasLimit         uint64            `json:"gas_limit"`         // Maximum gas this transaction is willing to consume
	GasPrice         uint64            `json:"gas_price"`         // Price per unit of gas in leaf units
}

// GetHash returns the hash of this transaction
func (t *Transaction) GetHash() Hash32 {
	h := sha256.New()
	h.Write(t.From[:])
	h.Write(t.To[:])
	h.Write(utils.Uint64ToBytes(t.Amount))
	h.Write(utils.Uint64ToBytes(t.Nonce))
	h.Write(utils.Uint64ToBytes(t.Timestamp))
	h.Write(utils.Uint64ToBytes(t.ChainID)) // Include ChainID for replay protection
	h.Write(utils.Uint64ToBytes(t.GasLimit)) // Include gas limit
	h.Write(utils.Uint64ToBytes(t.GasPrice)) // Include gas price

	// Hash the contract instructions if present
	if len(t.Instructions) > 0 {
		codeHash := ComputeCodeHash(t.Instructions)
		h.Write(codeHash[:])
	}

	// Hash the init instructions if present
	if len(t.InitInstructions) > 0 {
		initHash := ComputeCodeHash(t.InitInstructions)
		h.Write(initHash[:])
	}

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

// GetSignature computes the signature for this transaction using the given private key
func (t *Transaction) GetSignature(privateKey []byte) Signature {
	// Sign the transaction hash
	hash := t.GetHash()
	sig := ed25519.Sign(privateKey, hash[:])
	var signature Signature
	copy(signature[:], sig)
	return signature
}

type Hash32 [32]byte

// Custom JSON marshaling for Hash32
func (h Hash32) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(h[:]) + `"`), nil
}

func (h *Hash32) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON string for Hash32")
	}

	decoded, err := base64.StdEncoding.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return fmt.Errorf("failed to decode base64: %v", err)
	}

	if len(decoded) != 32 {
		return fmt.Errorf("invalid Hash32 size: got %d, want 32", len(decoded))
	}

	copy(h[:], decoded)
	return nil
}

type BlockHeader struct {
	Version      uint64    `json:"version"`
	PreviousHash Hash32    `json:"previous_hash"`
	Height       uint64    `json:"height"`
	Timestamp    uint64    `json:"timestamp"`
	Nonce        uint64    `json:"nonce"`
	MerkleRoot   Hash32    `json:"merkle_root"`     // Transaction merkle root
	StateRoot    Hash32    `json:"state_root"`      // Account state merkle root
	ReceiptRoot  Hash32    `json:"receipt_root"`    // Transaction receipts merkle root
	Bits         uint32    `json:"bits"`            // Target in compact format (Bitcoin-style)
	TotalWork    string    `json:"total_work"`      // Cumulative work from genesis to this block (big.Int as string)
	GasLimit     uint64    `json:"gas_limit"`       // Maximum gas allowed in this block
	GasUsed      uint64    `json:"gas_used"`        // Total gas used by all transactions
	Beneficiary  PublicKey `json:"beneficiary"`     // Miner/validator address receiving rewards
}

// GetHash returns the hash of this block header
func (h *BlockHeader) GetHash() Hash32 {
	hasher := sha256.New()
	hasher.Write(utils.Uint64ToBytes(h.Version))
	hasher.Write(h.PreviousHash[:])
	hasher.Write(utils.Uint64ToBytes(h.Height))
	hasher.Write(utils.Uint64ToBytes(h.Timestamp))
	hasher.Write(h.MerkleRoot[:])
	hasher.Write(h.StateRoot[:])
	hasher.Write(h.ReceiptRoot[:])
	hasher.Write(utils.Uint32ToBytes(h.Bits)) // Include Bits field!

	// Encode TotalWork as big.Int bytes, not string
	workInt := new(big.Int)
	workInt.SetString(h.TotalWork, 10)
	hasher.Write(workInt.Bytes())

	hasher.Write(utils.Uint64ToBytes(h.GasLimit))
	hasher.Write(utils.Uint64ToBytes(h.GasUsed))
	hasher.Write(h.Beneficiary[:])
	hasher.Write(utils.Uint64ToBytes(h.Nonce))

	var hash Hash32
	copy(hash[:], hasher.Sum(nil))
	return hash
}

type Block struct {
	Header       BlockHeader   `json:"header"`
	Transactions []Transaction `json:"transactions"`
}

// GetHash returns the hash of this block (which is the hash of its header)
func (b *Block) GetHash() Hash32 {
	return b.Header.GetHash()
}

type AccountState struct {
	Address      PublicKey         `json:"address"`
	Balance      uint64            `json:"balance"`
	Nonce        uint64            `json:"nonce"`
	Instructions []avm.Instruction `json:"instructions"`
	Persistent   map[string]string `json:"persistent"`
	StorageRoot  Hash32            `json:"storage_root"` // Merkle Patricia trie root of contract storage
	CodeHash     Hash32            `json:"code_hash"`    // Hash of the contract code (Instructions)
}

// GetHash computes the hash of this account state
func (s *AccountState) GetHash() Hash32 {
	h := sha256.New()
	h.Write(s.Address[:])
	h.Write(utils.Uint64ToBytes(s.Balance))
	h.Write(utils.Uint64ToBytes(s.Nonce))

	// Hash the instructions (contract code)
	if len(s.Instructions) > 0 {
		// Serialize instructions and hash them
		for _, instruction := range s.Instructions {
			h.Write([]byte{byte(instruction.Opcode)})
			if instruction.Value != nil {
				h.Write(instruction.Value.Bytes())
			}
			h.Write(utils.Uint32ToBytes(uint32(instruction.Param)))
		}
	}

	// Hash the persistent storage
	h.Write(s.StorageRoot[:])
	h.Write(s.CodeHash[:])

	var hash Hash32
	copy(hash[:], h.Sum(nil))
	return hash
}

type Chain struct {
	Blocks        []*Block                    `json:"blocks"`
	AccountStates map[PublicKey]*AccountState `json:"account_states"`
	Tip           *Block                      `json:"-"` // Points to latest block for O(1) access
	BlockIndex    map[Hash32]*Block           `json:"-"` // Hash -> Block lookup for O(1) parent access
}

// DeepCopy creates a deep copy of the chain for safe concurrent validation
func (c *Chain) DeepCopy() *Chain {
	if c == nil {
		return nil
	}

	// Copy blocks slice
	blocks := make([]*Block, len(c.Blocks))
	for i, block := range c.Blocks {
		// Blocks are immutable once created, so shallow copy is safe
		blocks[i] = block
	}

	// Deep copy account states map
	accountStates := make(map[PublicKey]*AccountState)
	for pubKey, state := range c.AccountStates {
		if state != nil {
			accountStates[pubKey] = &AccountState{
				Address: state.Address,
				Balance: state.Balance,
				Nonce:   state.Nonce,
			}
		}
	}

	// Deep copy block index
	blockIndex := make(map[Hash32]*Block)
	for hash, block := range c.BlockIndex {
		blockIndex[hash] = block // Blocks are immutable, shallow copy is safe
	}

	// Set tip pointer
	var tip *Block
	if len(blocks) > 0 {
		tip = blocks[len(blocks)-1]
	}

	return &Chain{
		Blocks:        blocks,
		AccountStates: accountStates,
		Tip:           tip,
		BlockIndex:    blockIndex,
	}
}

// AddBlock adds a block to the chain and updates tip/index
func (c *Chain) AddBlock(block *Block) {
	c.Blocks = append(c.Blocks, block)
	c.Tip = block

	if c.BlockIndex == nil {
		c.BlockIndex = make(map[Hash32]*Block)
	}

	blockHash := block.GetHash()
	c.BlockIndex[blockHash] = block
}

// GetBlockByHash returns a block by its hash in O(1) time
func (c *Chain) GetBlockByHash(hash Hash32) (*Block, bool) {
	if c.BlockIndex == nil {
		return nil, false
	}
	block, exists := c.BlockIndex[hash]
	return block, exists
}

// InitializeIndexes builds the block index and sets tip for existing chains
func (c *Chain) InitializeIndexes() {
	c.BlockIndex = make(map[Hash32]*Block)

	for _, block := range c.Blocks {
		blockHash := block.GetHash()
		c.BlockIndex[blockHash] = block
	}

	if len(c.Blocks) > 0 {
		c.Tip = c.Blocks[len(c.Blocks)-1]
	}
}
