package blockchain

import "crypto/ed25519"

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

type Transaction struct {
	From      PublicKey
	To        PublicKey
	Amount    uint64
	Signature Signature
	Nonce     uint64
}

type BlockHeader struct {
	Version      uint64
	PreviousHash [32]byte
	Timestamp    uint64
	Nonce        uint64
	MerkleRoot   [32]byte
}

type Block struct {
	Header       BlockHeader
	Transactions []Transaction
}

type AccountState struct {
	Address PublicKey
	Balance uint64
	Nonce   uint64
}

type Chain struct {
	Blocks        []*Block
	AccountStates map[PublicKey]*AccountState
}
