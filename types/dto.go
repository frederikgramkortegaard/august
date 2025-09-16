package types

// HandshakePayload is sent when nodes first connect
type HandshakePayload struct {
	NodeID      string `json:"node_id"`
	ChainHeight int    `json:"chain_height"`
	Version     string `json:"version"`
	ListenPort  string `json:"listen_port"` // The port this node is listening on
}

// NewBlockHeaderPayload broadcasts a new block header to peers (headers-first)
type NewBlockHeaderPayload struct {
	Header BlockHeader `json:"header"`
}

// NewBlockPayload broadcasts a new block to peers (full block, used for responses)
type NewBlockPayload struct {
	Block *Block `json:"block"`
}

// RequestBlockPayload requests a specific block
type RequestBlockPayload struct {
	BlockHash string `json:"block_hash"`
}

// NewTxPayload broadcasts a new transaction
type NewTxPayload struct {
	Transaction *Transaction `json:"transaction"`
}

// PingPayload for keepalive
type PingPayload struct {
	Timestamp int64 `json:"timestamp"`
}

// PongPayload response to ping
type PongPayload struct {
	Timestamp int64 `json:"timestamp"`
}

// RequestPeersPayload requests a list of known peers
type RequestPeersPayload struct {
	MaxPeers int `json:"max_peers"` // Maximum number of peers to return
}

// SharePeersPayload shares a list of known peer addresses
type SharePeersPayload struct {
	Peers []string `json:"peers"` // List of peer addresses
}

// RequestChainHeadPayload requests the current chain head info
type RequestChainHeadPayload struct {
	// Empty - just requesting current head info
}

// ChainHeadPayload represents the current chain head information
type ChainHeadPayload struct {
	HeadHash  string      `json:"head_hash"`
	Height    uint64      `json:"height"`
	TotalWork string      `json:"total_work"`
	Header    BlockHeader `json:"header"`
}

// RequestHeadersPayload requests block headers in a range or by specific hashes
type RequestHeadersPayload struct {
	StartHeight uint64   `json:"start_height,omitempty"`
	Count       uint64   `json:"count,omitempty"`      // Max number of headers to return
	StartHash   string   `json:"start_hash,omitempty"` // Alternative: start from hash (base64)
	Hashes      []string `json:"hashes,omitempty"`     // Alternative: specific header hashes (base64)
}

// HeadersPayload contains a batch of block headers
type HeadersPayload struct {
	Headers []BlockHeader `json:"headers"`
}

// RequestBlocksPayload requests full blocks in a range or by hashes
type RequestBlocksPayload struct {
	StartHeight uint64   `json:"start_height,omitempty"`
	Count       uint64   `json:"count,omitempty"`  // Max number of blocks
	Hashes      []string `json:"hashes,omitempty"` // Alternative: specific block hashes (base64)
}

// BlocksPayload contains a batch of full blocks
type BlocksPayload struct {
	Blocks []*Block `json:"blocks"`
}

// SubmitBlockPayload contains a mined block for submission
type SubmitBlockPayload struct {
	Block *Block `json:"block"`
}
