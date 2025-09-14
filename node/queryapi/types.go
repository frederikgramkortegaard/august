package queryapi

import "august/blockchain"

// BalanceResponse represents the response from the balance API endpoint
type BalanceResponse struct {
	Address string `json:"address"`
	Exists  bool   `json:"exists"`
	Balance uint64 `json:"balance"`
	Nonce   uint64 `json:"nonce"`
}

// SubmitTransactionResponse represents the response from submitting a transaction
type SubmitTransactionResponse struct {
	Status string `json:"status"`
	Hash   string `json:"hash"`
}

// TransactionResponse represents the response from the transaction lookup API
type TransactionResponse struct {
	Found       bool        `json:"found"`
	Transaction interface{} `json:"transaction,omitempty"`
	BlockHeight int         `json:"block_height,omitempty"`
	BlockHash   string      `json:"block_hash,omitempty"`
	TxIndex     int         `json:"tx_index,omitempty"`
	Status      string      `json:"status,omitempty"`
	InMempool   bool        `json:"in_mempool,omitempty"`
	Hash        string      `json:"hash,omitempty"`
}

// BlockResponse represents the response from the block lookup API
type BlockResponse struct {
	Found         bool        `json:"found"`
	Block         interface{} `json:"block,omitempty"`
	Height        int         `json:"height,omitempty"`
	Hash          string      `json:"hash,omitempty"`
	TxCount       int         `json:"tx_count,omitempty"`
	Confirmations int         `json:"confirmations,omitempty"`
}

// MempoolResponse represents the response from the mempool API
type MempoolResponse struct {
	Count        int                       `json:"count"`
	Transactions []blockchain.Transaction `json:"transactions"`
}