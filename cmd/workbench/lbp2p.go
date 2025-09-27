package main

import (
	"august/blockchain"
	"august/libnet"
	"august/libnet/protobuf"
	"august/networking"
	"august/node"
	"context"
	"crypto/rand"
	"fmt"
	"github.com/google/uuid"
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

	n1 := node.NewNode(node.NodeConfig{
		Port:      "9991",
		NodeID:    "myid",
		SeedPeers: []string{},
		QueryPort: "10001",
	})

	h1, peerInfo := setupSeedNode()
	h1.Node = n1
	addrs, _ := peerstore.AddrInfoToP2pAddrs(&peerInfo)

	fmt.Println("libp2p node address:", addrs[0])

	// Build out a new Host
	h2, _ := libnet.NewP2PHost("127.0.0.1", 9001, context.Background(), peerInfo)
	n2 := node.NewNode(node.NodeConfig{
		Port:      "9992",
		NodeID:    "myid2",
		SeedPeers: []string{},
		QueryPort: "10002",
	})
	h2.Node = n2

	if err := h2.Host.Connect(context.Background(), peerInfo); err != nil {
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
	s, _ := networking.StringToHash32("0000c8cca9d9f92d927f1c628f799ff68a5ac83317d00a803b579ff44f4351f6")
	hashes = append(hashes, s)

	messageID := uuid.New().String()
	ch := h2.AddMessageToPending(messageID)
	h2.Rpcprotocol.RequestHeaders(messageID, hashes)

	<-ch
	data := h2.GetDataFromMessage(messageID)
	for _, msg := range data {
		resp := msg.(*protobuf.HeadersResponse)
		for _, protoHeader := range resp.HeaderData {
			blockHeader := libnet.ProtoHeaderToBlockHeader(protoHeader)
			log.Print(blockHeader.GetHash())
		}
	}

	// Request a block
	messageID = uuid.New().String()
	ch = h2.AddMessageToPending(messageID)
	h2.Rpcprotocol.RequestBlocks(messageID, hashes)
	<-ch
	data = h2.GetDataFromMessage(messageID)
	for _, msg := range data {
		resp := msg.(*protobuf.BlocksResponse)
		for _, protoBlock := range resp.BlockData {
			block := libnet.ProtoBlockToBlock(protoBlock)
			log.Print(block.Header.PreviousHash.String())
		}
	}
	
}

