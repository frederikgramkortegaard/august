package networking

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// RequestResponse represents a message that supports request-response correlation
type RequestResponse interface {
	GetRequestID() string
	SetRequestID(id string)
	GetReplyTo() string
	SetReplyTo(id string)
}

// ReqRespConfig holds configuration for request-response handling
type ReqRespConfig struct {
	MaxResponseWaitTimeout time.Duration // How long to wait for responses
	MaxPendingRequests     int           // Maximum number of pending requests
}

// DefaultReqRespConfig returns sensible defaults for request-response handling
func DefaultReqRespConfig() ReqRespConfig {
	return ReqRespConfig{
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

// ReqRespClient handles request-response correlation and timeout management
type ReqRespClient struct {
	config          ReqRespConfig
	pendingRequests map[string]chan RequestResponse
	pendingMutex    sync.RWMutex
	sender          MessageSender
}

// NewReqRespClient creates a new request-response client
func NewReqRespClient(config ReqRespConfig, sender MessageSender) *ReqRespClient {
	return &ReqRespClient{
		config:          config,
		pendingRequests: make(map[string]chan RequestResponse),
		sender:          sender,
	}
}

// generateRequestID creates a unique request ID
func (c *ReqRespClient) generateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// SendRequest sends a request and waits for a response with correlation
func (c *ReqRespClient) SendRequest(peerAddress string, msg RequestResponse) (RequestResponse, error) {
	// Generate request ID and create response channel
	requestID := c.generateRequestID()
	responseChan := make(chan RequestResponse, 1)

	// Register the pending request
	c.pendingMutex.Lock()
	if len(c.pendingRequests) >= c.config.MaxPendingRequests {
		c.pendingMutex.Unlock()
		return nil, fmt.Errorf("too many pending requests")
	}
	c.pendingRequests[requestID] = responseChan
	c.pendingMutex.Unlock()

	// Clean up the pending request when done
	defer func() {
		c.pendingMutex.Lock()
		delete(c.pendingRequests, requestID)
		c.pendingMutex.Unlock()
	}()

	// Set request ID on the message
	msg.SetRequestID(requestID)

	// Send the message
	if err := c.sender.SendMessage(peerAddress, msg); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(c.config.MaxResponseWaitTimeout):
		return nil, fmt.Errorf("timeout waiting for response from %s", peerAddress)
	}
}

// HandleResponse processes an incoming response and delivers it to the waiting request
func (c *ReqRespClient) HandleResponse(msg RequestResponse) bool {
	replyTo := msg.GetReplyTo()
	if replyTo == "" {
		return false // Not a response
	}

	c.pendingMutex.RLock()
	responseChan, exists := c.pendingRequests[replyTo]
	c.pendingMutex.RUnlock()

	if !exists {
		return false // No pending request for this response
	}

	// Send the response to the waiting goroutine
	select {
	case responseChan <- msg:
		return true
	default:
		return false // Channel full
	}
}

// GetPendingRequestCount returns the number of currently pending requests
func (c *ReqRespClient) GetPendingRequestCount() int {
	c.pendingMutex.RLock()
	defer c.pendingMutex.RUnlock()
	return len(c.pendingRequests)
}

// SendNotification sends a fire-and-forget message without waiting for response
func (c *ReqRespClient) SendNotification(peerAddress string, msg RequestResponse) error {
	// Notifications don't need request IDs or correlation
	// Clear any existing request ID to indicate this is a notification
	msg.SetRequestID("")
	msg.SetReplyTo("")

	// Just send the message and return
	return c.sender.SendMessage(peerAddress, msg)
}