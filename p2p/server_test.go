package p2p

import (
	"encoding/json"
	"gocuria/blockchain/store"
	"net"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	chainStore := store.NewMemoryChainStore()
	config := Config{
		Port:   "0", // Use port 0 for testing (OS picks available port)
		NodeID: "test-node",
		Store:  chainStore,
	}

	server := NewServer(config)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.config.NodeID != "test-node" {
		t.Errorf("Expected NodeID 'test-node', got '%s'", server.config.NodeID)
	}

	if server.peerManager == nil {
		t.Error("PeerManager should be initialized")
	}
}

func TestServerStart(t *testing.T) {
	chainStore := store.NewMemoryChainStore()
	config := Config{
		Port:   "0", // Use port 0 for testing
		NodeID: "test-node",
		Store:  chainStore,
	}

	server := NewServer(config)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	if server.listener == nil {
		t.Error("Listener should be set after starting")
	}

	// Clean up
	defer server.Stop()

	// Get the actual port that was assigned
	addr := server.listener.Addr().(*net.TCPAddr)
	if addr.Port == 0 {
		t.Error("Server should be listening on a valid port")
	}
}

func TestServerAcceptConnection(t *testing.T) {
	chainStore := store.NewMemoryChainStore()
	config := Config{
		Port:   "0",
		NodeID: "server-node",
		Store:  chainStore,
	}

	server := NewServer(config)
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Get server address
	serverAddr := server.listener.Addr().String()

	// Connect to server
	conn, err := net.DialTimeout("tcp", serverAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Give server time to process the connection
	time.Sleep(100 * time.Millisecond)

	// Check that peer was added
	peers := server.peerManager.GetConnectedPeers()
	if len(peers) == 0 {
		t.Error("Expected at least one connected peer")
	}
}

func TestHandshakeExchange(t *testing.T) {
	chainStore := store.NewMemoryChainStore()
	config := Config{
		Port:   "0",
		NodeID: "handshake-node",
		Store:  chainStore,
	}

	server := NewServer(config)
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Connect to server
	serverAddr := server.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", serverAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Read handshake message from server
	decoder := json.NewDecoder(conn)
	var msg Message

	// Set read deadline to avoid hanging
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	err = decoder.Decode(&msg)
	if err != nil {
		t.Fatalf("Failed to decode handshake: %v", err)
	}

	if msg.Type != MessageTypeHandshake {
		t.Errorf("Expected handshake message, got %s", msg.Type)
	}

	var handshake HandshakePayload
	err = msg.ParsePayload(&handshake)
	if err != nil {
		t.Fatalf("Failed to parse handshake payload: %v", err)
	}

	if handshake.NodeID != "handshake-node" {
		t.Errorf("Expected NodeID 'handshake-node', got '%s'", handshake.NodeID)
	}

	if handshake.Version != "1.0" {
		t.Errorf("Expected Version '1.0', got '%s'", handshake.Version)
	}
}

func TestPingPongExchange(t *testing.T) {
	chainStore := store.NewMemoryChainStore()
	config := Config{
		Port:   "0",
		NodeID: "ping-node",
		Store:  chainStore,
	}

	server := NewServer(config)
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Connect to server
	serverAddr := server.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", serverAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	// Skip the handshake message from server
	var handshakeMsg Message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	decoder.Decode(&handshakeMsg)

	// Send ping
	timestamp := time.Now().Unix()
	pingPayload := PingPayload{Timestamp: timestamp}
	pingMsg, err := NewMessage(MessageTypePing, pingPayload)
	if err != nil {
		t.Fatalf("Failed to create ping message: %v", err)
	}

	err = encoder.Encode(pingMsg)
	if err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}

	// Read pong response
	var pongMsg Message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	err = decoder.Decode(&pongMsg)
	if err != nil {
		t.Fatalf("Failed to decode pong: %v", err)
	}

	if pongMsg.Type != MessageTypePong {
		t.Errorf("Expected pong message, got %s", pongMsg.Type)
	}

	var pongPayload PongPayload
	err = pongMsg.ParsePayload(&pongPayload)
	if err != nil {
		t.Fatalf("Failed to parse pong payload: %v", err)
	}

	// Pong should have a timestamp (not necessarily the same as ping)
	if pongPayload.Timestamp == 0 {
		t.Error("Pong timestamp should be set")
	}
}
