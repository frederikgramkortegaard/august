package p2p

import (
	"encoding/json"
	"gocuria/blockchain"
	"testing"
	"time"
)

func TestNewMessage(t *testing.T) {
	payload := HandshakePayload{
		NodeID:      "test-node-123",
		ChainHeight: 42,
		Version:     "1.0",
	}

	msg, err := NewMessage(MessageTypeHandshake, payload)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	if msg.Type != MessageTypeHandshake {
		t.Errorf("Expected type %s, got %s", MessageTypeHandshake, msg.Type)
	}

	if len(msg.Payload) == 0 {
		t.Error("Expected payload to be set")
	}
}

func TestMessageParsePayload(t *testing.T) {
	originalPayload := HandshakePayload{
		NodeID:      "test-node-456",
		ChainHeight: 100,
		Version:     "2.0",
	}

	msg, err := NewMessage(MessageTypeHandshake, originalPayload)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	var parsedPayload HandshakePayload
	err = msg.ParsePayload(&parsedPayload)
	if err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}

	if parsedPayload.NodeID != originalPayload.NodeID {
		t.Errorf("Expected NodeID %s, got %s", originalPayload.NodeID, parsedPayload.NodeID)
	}

	if parsedPayload.ChainHeight != originalPayload.ChainHeight {
		t.Errorf("Expected ChainHeight %d, got %d", originalPayload.ChainHeight, parsedPayload.ChainHeight)
	}

	if parsedPayload.Version != originalPayload.Version {
		t.Errorf("Expected Version %s, got %s", originalPayload.Version, parsedPayload.Version)
	}
}

func TestHandshakePayload(t *testing.T) {
	payload := HandshakePayload{
		NodeID:      "node-abc123",
		ChainHeight: 1337,
		Version:     "1.0",
	}

	msg, err := NewMessage(MessageTypeHandshake, payload)
	if err != nil {
		t.Fatalf("Failed to create handshake message: %v", err)
	}

	var parsed HandshakePayload
	err = msg.ParsePayload(&parsed)
	if err != nil {
		t.Fatalf("Failed to parse handshake: %v", err)
	}

	if parsed != payload {
		t.Error("Handshake payload not preserved through serialization")
	}
}

func TestNewBlockPayload(t *testing.T) {
	block := &blockchain.Block{
		Header: blockchain.BlockHeader{
			Version:   1,
			Timestamp: uint64(time.Now().Unix()),
			Nonce:     12345,
		},
		Transactions: []blockchain.Transaction{},
	}

	payload := NewBlockPayload{Block: block}

	msg, err := NewMessage(MessageTypeNewBlock, payload)
	if err != nil {
		t.Fatalf("Failed to create new block message: %v", err)
	}

	var parsed NewBlockPayload
	err = msg.ParsePayload(&parsed)
	if err != nil {
		t.Fatalf("Failed to parse new block: %v", err)
	}

	if parsed.Block.Header.Timestamp != block.Header.Timestamp {
		t.Errorf("Expected timestamp %d, got %d", block.Header.Timestamp, parsed.Block.Header.Timestamp)
	}
}

func TestPingPongPayload(t *testing.T) {
	timestamp := time.Now().Unix()

	// Test Ping
	pingPayload := PingPayload{Timestamp: timestamp}
	pingMsg, err := NewMessage(MessageTypePing, pingPayload)
	if err != nil {
		t.Fatalf("Failed to create ping message: %v", err)
	}

	var parsedPing PingPayload
	err = pingMsg.ParsePayload(&parsedPing)
	if err != nil {
		t.Fatalf("Failed to parse ping: %v", err)
	}

	if parsedPing.Timestamp != timestamp {
		t.Errorf("Expected timestamp %d, got %d", timestamp, parsedPing.Timestamp)
	}

	// Test Pong
	pongPayload := PongPayload{Timestamp: timestamp}
	pongMsg, err := NewMessage(MessageTypePong, pongPayload)
	if err != nil {
		t.Fatalf("Failed to create pong message: %v", err)
	}

	var parsedPong PongPayload
	err = pongMsg.ParsePayload(&parsedPong)
	if err != nil {
		t.Fatalf("Failed to parse pong: %v", err)
	}

	if parsedPong.Timestamp != timestamp {
		t.Errorf("Expected timestamp %d, got %d", timestamp, parsedPong.Timestamp)
	}
}

func TestRequestBlockPayload(t *testing.T) {
	payload := RequestBlockPayload{
		BlockHash: "abc123def456",
		Height:    42,
	}

	msg, err := NewMessage(MessageTypeRequestBlock, payload)
	if err != nil {
		t.Fatalf("Failed to create request block message: %v", err)
	}

	var parsed RequestBlockPayload
	err = msg.ParsePayload(&parsed)
	if err != nil {
		t.Fatalf("Failed to parse request block: %v", err)
	}

	if parsed.BlockHash != payload.BlockHash {
		t.Errorf("Expected hash %s, got %s", payload.BlockHash, parsed.BlockHash)
	}

	if parsed.Height != payload.Height {
		t.Errorf("Expected height %d, got %d", payload.Height, parsed.Height)
	}
}

func TestMessageSerialization(t *testing.T) {
	payload := HandshakePayload{
		NodeID:      "test-serialization",
		ChainHeight: 999,
		Version:     "1.0",
	}

	msg, err := NewMessage(MessageTypeHandshake, payload)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	// Serialize to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Deserialize from JSON
	var deserializedMsg Message
	err = json.Unmarshal(data, &deserializedMsg)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	// Check type is preserved
	if deserializedMsg.Type != MessageTypeHandshake {
		t.Errorf("Expected type %s, got %s", MessageTypeHandshake, deserializedMsg.Type)
	}

	// Check payload can be parsed
	var parsedPayload HandshakePayload
	err = deserializedMsg.ParsePayload(&parsedPayload)
	if err != nil {
		t.Fatalf("Failed to parse deserialized payload: %v", err)
	}

	if parsedPayload != payload {
		t.Error("Payload not preserved through JSON serialization")
	}
}
