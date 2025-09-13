package reqresp

import (
	"time"
)

// RequestResponse represents a message that supports request-response correlation
type RequestResponse interface {
	GetRequestID() string
	SetRequestID(id string)
	GetReplyTo() string
	SetReplyTo(id string)
}

// Config holds configuration for request-response handling
type Config struct {
	MaxResponseWaitTimeout time.Duration // How long to wait for responses
	MaxPendingRequests     int           // Maximum number of pending requests
}

// DefaultConfig returns sensible defaults for request-response handling
func DefaultConfig() Config {
	return Config{
		MaxResponseWaitTimeout: 5 * time.Second,
		MaxPendingRequests:     100,
	}
}

// PendingRequest tracks a request waiting for a response
type PendingRequest struct {
	RequestID    string
	ResponseChan chan RequestResponse
	CreatedAt    time.Time
}

// MessageSender defines the interface for sending messages over connections
type MessageSender interface {
	SendMessage(peerAddress string, msg RequestResponse) error
}
