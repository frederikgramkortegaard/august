package networking

import (
	"august/config"
	"august/utils"
	"encoding/json"
	"fmt"
	"time"
)

// SendRequest sends a request and waits for a response with correlation
func (s *Server) SendRequest(peer *Peer, msg *Message) (*Message, error) {
	// Generate request ID and create response channel
	requestID := utils.GenerateUUID()
	responseChan := make(chan *Message, 1)

	// Register the pending request
	s.pendingMutex.Lock()
	if len(s.pendingRequests) >= config.MaxPendingRequests {
		s.pendingMutex.Unlock()
		return nil, fmt.Errorf("too many pending requests")
	}
	s.pendingRequests[requestID] = responseChan
	s.pendingMutex.Unlock()

	// Clean up the pending request when done
	defer func() {
		s.pendingMutex.Lock()
		delete(s.pendingRequests, requestID)
		s.pendingMutex.Unlock()
	}()

	// Set request ID on the message
	msg.RequestID = requestID

	// Send the message
	if err := s.sendMessageToPeer(peer.Address, msg); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(config.RequestTimeout):
		return nil, fmt.Errorf("timeout waiting for response from %s", peer.Address)
	}
}

// SendNotification sends a fire-and-forget message without waiting for response
func (s *Server) SendNotification(peer *Peer, msg *Message) error {
	// Notifications don't need request IDs or correlation
	msg.RequestID = ""
	msg.ReplyTo = ""

	// Just send the message and return
	return s.sendMessageToPeer(peer.Address, msg)
}

// Reply sends a response to a previous request
func (s *Server) Reply(peer *Peer, msg *Message, originalRequestID string) error {
	// Set the reply-to field to correlate with the original request
	msg.ReplyTo = originalRequestID

	// Clear the request ID since this is a response, not a new request
	msg.RequestID = ""

	// Send the response message
	return s.sendMessageToPeer(peer.Address, msg)
}

// HandleResponse processes an incoming response and delivers it to the waiting request
func (s *Server) HandleResponse(msg *Message) bool {
	if msg.ReplyTo == "" {
		return false // Not a response
	}

	s.pendingMutex.RLock()
	responseChan, exists := s.pendingRequests[msg.ReplyTo]
	s.pendingMutex.RUnlock()

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
func (s *Server) GetPendingRequestCount() int {
	s.pendingMutex.RLock()
	defer s.pendingMutex.RUnlock()
	return len(s.pendingRequests)
}

// sendMessageToPeer sends a message to a peer (private helper for reqresp methods)
func (s *Server) sendMessageToPeer(peerAddress string, msg *Message) error {
	// Get the connection for this peer
	s.peerConnectionsMu.RLock()
	conn, exists := s.peerConnections[peerAddress]
	s.peerConnectionsMu.RUnlock()

	if !exists {
		return fmt.Errorf("no connection to peer %s", peerAddress)
	}

	// Send the message over the connection
	encoder := json.NewEncoder(conn)
	err := encoder.Encode(msg)
	if err != nil {
		s.logf("Failed to encode/send message %s: %v", msg.Type, err)
	}
	return err
}