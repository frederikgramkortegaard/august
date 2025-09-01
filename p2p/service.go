package p2p

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"
	
	"gocuria/blockchain"
)

// RelayBlock broadcasts a block to all connected peers, optionally excluding a specific peer
// Returns a completion channel that will be closed when the relay operation completes
func RelayBlock(server *Server, block *blockchain.Block, excludePeerAddr string) <-chan struct{} {
	complete := make(chan struct{})
	
	go func() {
		defer close(complete)
		
		blockHash := blockchain.HashBlockHeader(&block.Header)
		
		// Add to recent blocks to prevent duplication when we receive it back from peers
		// (only when not excluding anyone, meaning this is a local block)
		if excludePeerAddr == "" {
			server.recentBlocksMu.Lock()
			server.recentBlocks[blockHash] = time.Now()
			server.recentBlocksMu.Unlock()
		}
		
		server.logf("Relaying block %x to other peers (excluding %s)", blockHash[:8], excludePeerAddr)

		// Create the block payload
		blockPayload := NewBlockPayload{Block: block}
		msg, err := NewMessage(MessageTypeNewBlock, blockPayload)
		if err != nil {
			server.logf("Failed to create relay message for block %x: %v", blockHash[:8], err)
			return
		}

		// Send to all connected peers except the sender
		relayCount := 0
		for _, peer := range server.peerManager.GetConnectedPeers() {
			if peer.Address != excludePeerAddr && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := server.reqRespClient.SendNotification(peer.Address, msg); err != nil {
					server.logf("Failed to relay block %x to peer %s: %v", blockHash[:8], peer.Address, err)
				} else {
					server.logf("Relayed block %x to peer %s", blockHash[:8], peer.Address)
					relayCount++
				}
			}
		}

		server.logf("Successfully relayed block %x to %d peers", blockHash[:8], relayCount)
	}()
	
	return complete
}

// BroadcastTransaction broadcasts a transaction to all connected peers
// Returns a completion channel that will be closed when the broadcast operation completes
func BroadcastTransaction(server *Server, tx *blockchain.Transaction) <-chan struct{} {
	return broadcastTransactionToAllExcept(server, tx, "")
}

// broadcastTransactionToAllExcept broadcasts a transaction to all connected peers except the specified one
// Returns a completion channel that will be closed when the broadcast operation completes
func broadcastTransactionToAllExcept(server *Server, tx *blockchain.Transaction, excludePeerAddr string) <-chan struct{} {
	complete := make(chan struct{})
	
	go func() {
		defer close(complete)
		
		// Create the transaction payload
		txPayload := NewTxPayload{Transaction: tx}
		msg, err := NewMessage(MessageTypeNewTx, txPayload)
		if err != nil {
			server.logf("Failed to create transaction message: %v", err)
			return
		}

		// Log what we're doing
		if excludePeerAddr == "" {
			server.logf("Broadcasting transaction to all peers")
		} else {
			server.logf("Relaying transaction to other peers (excluding %s)", excludePeerAddr)
		}

		// Send to all connected peers
		sentCount := 0
		for _, peer := range server.peerManager.GetConnectedPeers() {
			if peer.Address != excludePeerAddr && peer.Status == PeerConnected {
				// Use SendNotification for fire-and-forget broadcast
				if err := server.reqRespClient.SendNotification(peer.Address, msg); err != nil {
					server.logf("Failed to send transaction to peer %s: %v", peer.Address, err)
				} else {
					server.logf("Sent transaction to peer %s", peer.Address)
					sentCount++
				}
			}
		}

		server.logf("Successfully sent transaction to %d peers", sentCount)
	}()
	
	return complete
}


// RequestBlockFromPeer requests a block and waits for the response
func RequestBlockFromPeer(server *Server, peerAddress string, blockHash string) (*blockchain.Block, error) {
	requestPayload := RequestBlockPayload{BlockHash: blockHash}
	
	msg, err := NewMessage(MessageTypeRequestBlock, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request message: %w", err)
	}
	
	response, err := server.reqRespClient.SendRequest(peerAddress, msg)
	if err != nil {
		return nil, err
	}
	
	// Cast back to Message and parse the response as a NewBlock message
	responseMsg := response.(*Message)
	if responseMsg.Type != MessageTypeNewBlock {
		return nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type)
	}
	
	var blockPayload NewBlockPayload
	if err := responseMsg.ParsePayload(&blockPayload); err != nil {
		return nil, fmt.Errorf("failed to parse block response: %w", err)
	}
	
	server.logf("Received block %s from %s", blockHash, peerAddress)
	return blockPayload.Block, nil
}

// RequestPeersFromPeer requests peers and waits for the response
func RequestPeersFromPeer(server *Server, peerAddress string, maxPeers int) ([]string, error) {
	requestPayload := RequestPeersPayload{MaxPeers: maxPeers}
	
	msg, err := NewMessage(MessageTypeRequestPeers, requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request message: %w", err)
	}
	
	response, err := server.reqRespClient.SendRequest(peerAddress, msg)
	if err != nil {
		return nil, err
	}
	
	// Cast back to Message and parse the response as a SharePeers message
	responseMsg := response.(*Message)
	if responseMsg.Type != MessageTypeSharePeers {
		return nil, fmt.Errorf("unexpected response type: %s", responseMsg.Type)
	}
	
	var sharePayload SharePeersPayload
	if err := responseMsg.ParsePayload(&sharePayload); err != nil {
		return nil, fmt.Errorf("failed to parse peers response: %w", err)
	}
	
	server.logf("Received %d peers from %s", len(sharePayload.Peers), peerAddress)
	return sharePayload.Peers, nil
}

// ProcessBlock attempts to add a block to the main chain, handling orphans
// Returns a completion channel that will be closed when processing completes
// excludePeerAddr: if provided, this peer will be excluded from relay (used when block came from a peer)
func ProcessBlock(server *Server, block *blockchain.Block, excludePeerAddr ...string) <-chan struct{} {
	complete := make(chan struct{})
	
	go func() {
		defer close(complete)
		
		blockHash := blockchain.HashBlockHeader(&block.Header)

		// Get current chain for validation
		chain, err := server.config.Store.GetChain()
		if err != nil {
			server.logf("Failed to get chain for block %x: %v", blockHash[:8], err)
			return
		}

		// Try to validate and add block directly to main chain
		if err := blockchain.ValidateAndApplyBlock(block, chain); err != nil {
			// Check if this is a missing parent error (orphan block)
			if missingParentErr, ok := err.(blockchain.ErrMissingParent); ok {
				server.logf("Block %x is orphan, missing parent %x. Adding to orphan pool.", 
					blockHash[:8], missingParentErr.Hash[:8])

				// Store in orphan pool
				server.orphanPoolMu.Lock()
				server.orphanPool[blockHash] = block
				server.orphanPoolMu.Unlock()

				// Request missing parent block from peers
				server.logf("Need to request parent block %x from peers", missingParentErr.Hash[:8])

				connectedPeers := server.peerManager.GetConnectedPeers()
				hashString := base64.StdEncoding.EncodeToString(missingParentErr.Hash[:])
				for _, peer := range connectedPeers {
					go func(peerAddr string) { 
						_, _ = RequestBlockFromPeer(server, peerAddr, hashString) 
					}(peer.Address)
				}
				return
			}

			// Other validation errors
			server.logf("Block %x validation failed: %v", blockHash[:8], err)
			return
		}

		// Block validation succeeded, now persist it to the store
		if err := server.config.Store.AddBlock(block); err != nil {
			server.logf("Failed to persist block %x to store: %v", blockHash[:8], err)
			return
		}

		// Block successfully added to main chain
		log.Printf("Block %x added to main chain", blockHash[:8])

		// Relay the block to connected peers (exclude sender if provided)
		var excludeAddr string
		if len(excludePeerAddr) > 0 {
			excludeAddr = excludePeerAddr[0]
		}
		go func() { <-RelayBlock(server, block, excludeAddr) }()

		// Try to connect any orphan blocks that might now be connectible
		tryConnectOrphans(server)
	}()
	
	return complete
}

// tryConnectOrphans attempts to connect orphan blocks to the main chain
func tryConnectOrphans(server *Server) {
	connected := true

	// Keep trying until no more orphans can be connected
	for connected {
		connected = false

		// Get current chain state for each attempt
		chain, err := server.config.Store.GetChain()
		if err != nil {
			server.logf("Failed to get chain for orphan connection: %v", err)
			return
		}

		server.orphanPoolMu.Lock()
		// Check each orphan block
		for orphanHash, orphanBlock := range server.orphanPool {
			// Try to validate and add this orphan block
			if err := blockchain.ValidateAndApplyBlock(orphanBlock, chain); err == nil {
				// Validation succeeded, now persist to store
				if err := server.config.Store.AddBlock(orphanBlock); err != nil {
					server.logf("Failed to persist orphan block %x to store: %v", orphanHash[:8], err)
					continue
				}

				// Successfully connected!
				server.logf("Connected orphan block %x to main chain", orphanHash[:8])

				// Remove from orphan pool
				delete(server.orphanPool, orphanHash)
				connected = true

				// Don't continue iterating as we modified the map
				break
			}
		}
		server.orphanPoolMu.Unlock()
	}

	server.orphanPoolMu.RLock()
	orphanCount := len(server.orphanPool)
	server.orphanPoolMu.RUnlock()
	
	if orphanCount > 0 {
		server.logf("Still have %d orphan blocks waiting for parents", orphanCount)
	}
}

// GetOrphanCount returns the number of orphan blocks waiting for parents (for testing)
func GetOrphanCount(server *Server) int {
	server.orphanPoolMu.RLock()
	defer server.orphanPoolMu.RUnlock()
	return len(server.orphanPool)
}