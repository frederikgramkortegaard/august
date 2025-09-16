package networking

import (
	. "august/types"
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
