package store

import (
	"errors"
	"fmt"
	"gocuria/blockchain"
)

type MemoryChainStore struct {
	chain *blockchain.Chain
}

func NewMemoryChainStore() *MemoryChainStore {
	return &MemoryChainStore{
		chain: &blockchain.Chain{
			Blocks:        make([]*blockchain.Block, 0),
			AccountStates: make(map[blockchain.PublicKey]*blockchain.AccountState),
		},
	}
}

func (m *MemoryChainStore) AddBlock(block *blockchain.Block) error {

	head, err := m.GetHeadBlock()
	if err != nil {
		return err
	}

	fmt.Println(head)

	accountStates, err := m.GetAccountStates()
	if err != nil {
		return err
	}

	if !blockchain.ValidateAndApplyBlock(block, head, accountStates) {
		return errors.New("could not validate block")
	}

	chain, err := m.GetChain()
	if err != nil {
		return err
	}
	if chain == nil {
		return errors.New("chain is nil")
	}
	chain.Blocks = append(chain.Blocks, block)

	return nil
}

func (m *MemoryChainStore) GetHeadBlock() (*blockchain.Block, error) {

	chain, err := m.GetChain()
	if err != nil {
		return nil, err
	}

	if chain == nil {
		return nil, errors.New("chain is nil")
	}

	// Not returning an err as nil checks on this is valid
	if len(chain.Blocks) < 1 {
		return nil, nil
	}

	return chain.Blocks[len(chain.Blocks)-1], nil
}

func (m *MemoryChainStore) GetAccountState(pubkey blockchain.PublicKey) (*blockchain.AccountState, error) {

	chain, err := m.GetChain()
	if err != nil {
		return nil, err
	}

	if chain == nil {
		return nil, errors.New("chain is nil")
	}

	res, ok := chain.AccountStates[pubkey]
	if !ok {
		return nil, errors.New("account state for key not found")
	}

	return res, nil
}

func (m *MemoryChainStore) GetChain() (*blockchain.Chain, error) {
	return m.chain, nil
}

func (m *MemoryChainStore) GetAccountStates() (map[blockchain.PublicKey]*blockchain.AccountState, error) {

	chain, err := m.GetChain()
	if err != nil {
		return nil, err
	}

	if chain == nil {
		return nil, errors.New("chain is nil")
	}

	return chain.AccountStates, nil
}

func (m *MemoryChainStore) GetBlockByHash(hash [32]byte) (*blockchain.Block, error) {

	chain, err := m.GetChain()
	if err != nil {
		return nil, err
	}

	if chain == nil {
		return nil, errors.New("chain is nil")
	}

	for _, block := range chain.Blocks {
		if blockchain.HashBlockHeader(&block.Header) == hash {
			return block, nil
		}

	}

	fmt.Println("could not find block with hash %x\n", hash)
	return nil, nil

}
