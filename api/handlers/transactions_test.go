package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"gocuria/blockchain"
	"gocuria/blockchain/store"
)

func TestHandleTransactions(t *testing.T) {
	// Create test store with genesis block
	testStore := store.NewMemoryChainStore()
	err := testStore.AddBlock(blockchain.GenesisBlock)
	if err != nil {
		t.Fatalf("Failed to add genesis block: %v", err)
	}

	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
		expectedInBody string
	}{
		{
			name:   "valid POST with signature validation failure",
			method: "POST",
			body: map[string]interface{}{
				"from":      "mT/mo20Z7VJ4kKdpjpQMPBxinKBkQ4N4ccsO0ZQ1Ips=", // FirstUser base64
				"to":        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",      // Zero recipient
				"amount":    1000,
				"nonce":     1,
				"signature": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==", // Zero signature
			},
			expectedStatus: 400,
			expectedInBody: "invalid transaction signature",
		},
		{
			name:   "signature validation fails first - realistic test",
			method: "POST",
			body: map[string]interface{}{
				"from":      "mT/mo20Z7VJ4kKdpjpQMPBxinKBkQ4N4ccsO0ZQ1Ips=",
				"to":        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
				"amount":    999999999999, // Would fail balance check, but signature fails first
				"nonce":     1,
				"signature": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==",
			},
			expectedStatus: 400,
			expectedInBody: "invalid transaction signature", // This is what actually happens
		},
		{
			name:           "method not allowed",
			method:         "GET",
			body:           nil,
			expectedStatus: 405,
			expectedInBody: "Method not allowed",
		},
		{
			name:           "invalid JSON",
			method:         "POST",
			body:           "invalid json",
			expectedStatus: 400,
			expectedInBody: "Invalid JSON format",
		},
		{
			name:   "coinbase transaction should be valid",
			method: "POST",
			body: map[string]interface{}{
				"from":      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", // Empty sender = coinbase
				"to":        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
				"amount":    1000,
				"nonce":     0,
				"signature": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==",
			},
			expectedStatus: 200,
			expectedInBody: "valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare request body
			var reqBody bytes.Buffer
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					// Handle raw string body (for invalid JSON test)
					reqBody.WriteString(str)
				} else {
					// Handle JSON body
					if err := json.NewEncoder(&reqBody).Encode(tt.body); err != nil {
						t.Fatalf("Failed to encode request body: %v", err)
					}
				}
			}

			// Create request
			req := httptest.NewRequest(tt.method, "/api/transactions", &reqBody)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler
			HandleTransactions(w, req, testStore)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check response body contains expected text
			if tt.expectedInBody != "" {
				responseBody := w.Body.String()
				if !strings.Contains(responseBody, tt.expectedInBody) {
					t.Errorf("Expected response body to contain %q, got %q", tt.expectedInBody, responseBody)
				}
			}
		})
	}
}

func TestHandleValidateTransaction(t *testing.T) {
	// Create minimal test for the internal function
	testStore := store.NewMemoryChainStore()
	testStore.AddBlock(blockchain.GenesisBlock)

	// Test with valid coinbase transaction
	validCoinbase := map[string]interface{}{
		"from":      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", // Coinbase
		"to":        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"amount":    1000,
		"nonce":     0,
		"signature": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==",
	}

	body, _ := json.Marshal(validCoinbase)
	req := httptest.NewRequest("POST", "/api/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handleValidateTransaction(w, req, testStore)

	if w.Code != 200 {
		t.Errorf("Expected status 200 for valid coinbase, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "valid" {
		t.Errorf("Expected status 'valid', got %q", response["status"])
	}
}