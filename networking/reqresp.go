package networking

import (
	"august/config"
	"august/utils"
	"fmt"
	"net"
	"time"
)

// SendRequest sends a message and waits for a response
func (n *Network) SendRequest(peerAddr string, msg *Message) (*Message, error) {
	// Use timeout from config
	timeout := config.RequestTimeout

	// Generate request ID
	requestID := utils.GenerateUUID()
	msg.RequestID = requestID

	// Create response channel
	respChan := make(chan *Message, 1)

	// Register pending request
	n.pendingMu.Lock()
	n.pendingRequests[requestID] = respChan
	n.pendingMu.Unlock()

	// Clean up when done
	defer func() {
		n.pendingMu.Lock()
		delete(n.pendingRequests, requestID)
		n.pendingMu.Unlock()
	}()

	// Get connection and send
	n.peerConnectionsMu.RLock()
	conn, exists := n.peerConnections[peerAddr]
	n.peerConnectionsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no connection to peer %s", peerAddr)
	}

	if err := n.sendMessage(conn, msg); err != nil {
		return nil, fmt.Errorf("send failed: %w", err)
	}

	// Wait for response with timeout
	select {
	case response := <-respChan:
		return response, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("request timeout")
	}
}

// SendNotification sends a message without waiting for response (fire-and-forget)
func (n *Network) SendNotification(peerAddr string, msg *Message) error {
	msg.RequestID = "" // No correlation needed
	msg.ReplyTo = ""

	n.peerConnectionsMu.RLock()
	conn, exists := n.peerConnections[peerAddr]
	n.peerConnectionsMu.RUnlock()

	if !exists {
		return fmt.Errorf("no connection to peer %s", peerAddr)
	}

	return n.sendMessage(conn, msg)
}

// HandleResponse routes incoming responses to waiting requests
func (n *Network) HandleResponse(msg *Message) bool {
	if msg.ReplyTo == "" {
		return false // Not a response
	}

	n.pendingMu.RLock()
	respChan, exists := n.pendingRequests[msg.ReplyTo]
	n.pendingMu.RUnlock()

	if !exists {
		return false // No one waiting for this response
	}

	select {
	case respChan <- msg:
		return true
	default:
		return false // Channel full
	}
}

// Reply sends a response to a request
func (n *Network) Reply(conn net.Conn, originalMsg *Message, response *Message) error {
	response.ReplyTo = originalMsg.RequestID
	return n.sendMessage(conn, response)
}

