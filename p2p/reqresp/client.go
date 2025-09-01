package reqresp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Client handles request-response correlation and timeout management
type Client struct {
	config          Config
	pendingRequests map[string]chan RequestResponse
	pendingMutex    sync.RWMutex
	sender          MessageSender
}

// NewClient creates a new request-response client
func NewClient(config Config, sender MessageSender) *Client {
	return &Client{
		config:          config,
		pendingRequests: make(map[string]chan RequestResponse),
		sender:          sender,
	}
}

// generateRequestID creates a unique request ID
func (c *Client) generateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// SendRequest sends a request and waits for a response with correlation
func (c *Client) SendRequest(peerAddress string, msg RequestResponse) (RequestResponse, error) {
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
func (c *Client) HandleResponse(msg RequestResponse) bool {
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
func (c *Client) GetPendingRequestCount() int {
	c.pendingMutex.RLock()
	defer c.pendingMutex.RUnlock()
	return len(c.pendingRequests)
}