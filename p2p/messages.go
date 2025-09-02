package p2p

import (
	"encoding/json"
	"gocuria/blockchain"
)

// MessageType defines the type of P2P message
type MessageType string

const (
	MessageTypeHandshake      MessageType = "handshake"
	MessageTypeNewBlockHeader MessageType = "new_block_header" // Headers-first gossip
	MessageTypeNewBlock       MessageType = "new_block"        // Full block (for requests only)
	MessageTypeRequestBlock   MessageType = "request_block"
	MessageTypeNewTx          MessageType = "new_transaction"
	MessageTypePing           MessageType = "ping"
	MessageTypePong           MessageType = "pong"
	MessageTypeRequestPeers   MessageType = "request_peers"
	MessageTypeSharePeers     MessageType = "share_peers"

	// Chain synchronization messages
	MessageTypeRequestChainHead MessageType = "request_chain_head"
	MessageTypeChainHead        MessageType = "chain_head"
	MessageTypeRequestHeaders   MessageType = "request_headers"
	MessageTypeHeaders          MessageType = "headers"
	MessageTypeRequestBlocks    MessageType = "request_blocks" // Batch blocks request
	MessageTypeBlocks           MessageType = "blocks"         // Batch blocks response
)

// Message represents a P2P message between nodes
type Message struct {
	Type      MessageType     `json:"type"`
	RequestID string          `json:"request_id,omitempty"` // For correlating requests
	ReplyTo   string          `json:"reply_to,omitempty"`   // For correlating responses
	Payload   json.RawMessage `json:"payload"`
}

// HandshakePayload is sent when nodes first connect
type HandshakePayload struct {
	NodeID      string `json:"node_id"`
	ChainHeight int    `json:"chain_height"`
	Version     string `json:"version"`
	ListenPort  string `json:"listen_port"` // The port this node is listening on
}

// NewBlockHeaderPayload broadcasts a new block header to peers (headers-first)
type NewBlockHeaderPayload struct {
	Header blockchain.BlockHeader `json:"header"`
}

// NewBlockPayload broadcasts a new block to peers (full block, used for responses)
type NewBlockPayload struct {
	Block *blockchain.Block `json:"block"`
}

// RequestBlockPayload requests a specific block
type RequestBlockPayload struct {
	BlockHash string `json:"block_hash"`
}

// NewTxPayload broadcasts a new transaction
type NewTxPayload struct {
	Transaction *blockchain.Transaction `json:"transaction"`
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

// ChainHeadPayload contains information about the current chain head
type ChainHeadPayload struct {
	Height    uint64                 `json:"height"`
	HeadHash  string                 `json:"head_hash"`  // Base64 encoded
	TotalWork string                 `json:"total_work"` // Big.Int as string
	Header    blockchain.BlockHeader `json:"header"`     // Full header for validation
}

// RequestHeadersPayload requests block headers in a range
type RequestHeadersPayload struct {
	StartHeight uint64 `json:"start_height"`
	Count       uint64 `json:"count"`      // Max number of headers to return
	StartHash   string `json:"start_hash"` // Alternative: start from hash (base64)
}

// HeadersPayload contains a batch of block headers
type HeadersPayload struct {
	Headers []blockchain.BlockHeader `json:"headers"`
}

// RequestBlocksPayload requests full blocks in a range or by hashes
type RequestBlocksPayload struct {
	StartHeight uint64   `json:"start_height,omitempty"`
	Count       uint64   `json:"count,omitempty"`  // Max number of blocks
	Hashes      []string `json:"hashes,omitempty"` // Alternative: specific block hashes (base64)
}

// BlocksPayload contains a batch of full blocks
type BlocksPayload struct {
	Blocks []*blockchain.Block `json:"blocks"`
}

// NewMessage creates a new P2P message with the given type and payload
func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:    msgType,
		Payload: json.RawMessage(payloadBytes),
	}, nil
}

// ParsePayload unmarshals the message payload into the provided interface
func (m *Message) ParsePayload(payload interface{}) error {
	return json.Unmarshal(m.Payload, payload)
}

// RequestResponse interface implementation
func (m *Message) GetRequestID() string {
	return m.RequestID
}

func (m *Message) SetRequestID(id string) {
	m.RequestID = id
}

func (m *Message) GetReplyTo() string {
	return m.ReplyTo
}

func (m *Message) SetReplyTo(id string) {
	m.ReplyTo = id
}
