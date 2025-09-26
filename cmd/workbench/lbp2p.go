package main

import (
	"august/blockchain"
	"august/libnet"
	"context"
	"crypto/rand"
	"fmt"
	peerstore "github.com/libp2p/go-libp2p/core/peer"
	"log"
)

func setupSeedNode() (*libnet.P2PHost, peerstore.AddrInfo) {

	node, _ := libnet.NewP2PHost("127.0.0.1", 9000, context.Background())

	peerInfo := peerstore.AddrInfo{
		ID:    node.Host.ID(),
		Addrs: node.Host.Addrs(),
	}

	return node, peerInfo

}

func main() {

	_, peerInfo := setupSeedNode()
	addrs, _ := peerstore.AddrInfoToP2pAddrs(&peerInfo)

	fmt.Println("libp2p node address:", addrs[0])
	var hosts []*libnet.P2PHost
	for i := range 1 {

		// Build out a new Host
		h, _ := libnet.NewP2PHost("127.0.0.1", 9001+i, context.Background(), peerInfo)

		hosts = append(hosts, h)

		if err := hosts[i].Host.Connect(context.Background(), peerInfo); err != nil {
			panic(err)
		}

		// Request some dummy hashes
		var hashes []blockchain.Hash32
		for range 5 {
			var h blockchain.Hash32
			_, err := rand.Read(h[:]) // fill the full 32 bytes
			if err != nil {
				log.Fatal(err)
			}
			hashes = append(hashes, h)
		}

		messageID, ech := h.Rpcprotocol.RequestHeaders(hashes)

		ch := make(chan struct{}, 1)

		h.PendingRequestsMu.Lock()
		msgs := h.PendingRequestsWaiters[messageID]
		if len(msgs) <= 0 {
			h.PendingRequestsWaiters[messageID] = append(h.PendingRequestsWaiters[messageID], ch)
			ch <- struct{}{}
		}
		h.PendingRequestsMu.Unlock()

		if <-ech != nil {
			log.Fatal("Received Error from Request", messageID)
		}
		<-ch
		h.PendingRequestsMu.Lock()
		log.Printf("Received Answers to Request: %s, %v", messageID, h.PendingRequestsWaiters[messageID][0], h.PendingRequests[messageID][0])
		h.PendingRequestsMu.Unlock()

	}

}
