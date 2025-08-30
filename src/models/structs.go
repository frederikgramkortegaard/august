package models

import "crypto/ed25519"

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
