package p2p

import (
	"encoding/json"
	"gocuria/blockchain"
)

// MessageType defines the type of P2P message
type MessageType string

const (
	MessageTypeHandshake    MessageType = "handshake"
	MessageTypeNewBlock     MessageType = "new_block"
	MessageTypeRequestBlock MessageType = "request_block"
	MessageTypeNewTx        MessageType = "new_transaction"
	MessageTypePing         MessageType = "ping"
	MessageTypePong         MessageType = "pong"
)

// Message represents a P2P message between nodes
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// HandshakePayload is sent when nodes first connect
type HandshakePayload struct {
	NodeID      string `json:"node_id"`
	ChainHeight int    `json:"chain_height"`
	Version     string `json:"version"`
}

// NewBlockPayload broadcasts a new block to peers
type NewBlockPayload struct {
	Block *blockchain.Block `json:"block"`
}

// RequestBlockPayload requests a specific block
type RequestBlockPayload struct {
	BlockHash string `json:"block_hash"`
	Height    int    `json:"height"`
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
