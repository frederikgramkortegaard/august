package networking

import (
	"encoding/json"
)

// NewMessage creates a new network message with the given type and payload
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

// MessageType defines the type of network message
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

	// Mining submission (one-off, no peer relationship required)
	MessageTypeSubmitBlock MessageType = "submit_block" // Miner block submission
)

// Message represents a network message between nodes
type Message struct {
	Type      MessageType     `json:"type"`
	RequestID string          `json:"request_id,omitempty"` // For correlating requests
	ReplyTo   string          `json:"reply_to,omitempty"`   // For correlating responses
	Payload   json.RawMessage `json:"payload"`
}

// ParsePayload unmarshals the message payload into the provided interface
func (m *Message) ParsePayload(payload interface{}) error {
	return json.Unmarshal(m.Payload, payload)
}
