package main

import (
	"august/blockchain"
	"august/libnet"
	"august/libnet/protobuf"
	"august/libnet/rpc"
	"august/networking"
	"august/node"
	"context"
	"crypto/rand"
	"fmt"
	"github.com/google/uuid"
	peerstore "github.com/libp2p/go-libp2p/core/peer"
	"log"
	"strconv"
)

func MakeNode(id int, seeds ...peerstore.AddrInfo) (*libnet.PeerService, *node.Node, *rpc.RPCProtocol, peerstore.AddrInfo) {
	n := node.NewNode(node.NodeConfig{
		Port:      strconv.Itoa(9990 + id),
		NodeID:    "node" + strconv.Itoa(id),
		SeedPeers: []string{},
		QueryPort: strconv.Itoa(10000 + id),
	})

	var rpcProto *rpc.RPCProtocol
	ps, _ := libnet.NewPeerService("127.0.0.1", 9000+id, context.Background(), func(ps *libnet.PeerService) {
		rpcProto = rpc.NewRPCProtocol(ps)
	}, seeds...)
	ps.Node = n

	peerInfo := peerstore.AddrInfo{
		ID:    ps.Host.ID(),
		Addrs: ps.Host.Addrs(),
	}

	return ps, n, rpcProto, peerInfo
}

func main() {
	// Create first peer
	h1, _, _, peerInfo1 := MakeNode(0)
	addrs, _ := peerstore.AddrInfoToP2pAddrs(&peerInfo1)
	fmt.Println("libp2p node address:", addrs[0])

	// Create second peer connected to first
	h2, _, rpc2, _ := MakeNode(1, peerInfo1)

	if err := h2.Host.Connect(context.Background(), peerInfo1); err != nil {
		panic(err)
	}

	// Setup some dummy request data
	var hashes []blockchain.Hash32
	for range 5 {
		var h blockchain.Hash32
		_, err := rand.Read(h[:])
		if err != nil {
			log.Fatal(err)
		}
		hashes = append(hashes, h)
	}
	s, _ := networking.StringToHash32("0000c8cca9d9f92d927f1c628f799ff68a5ac83317d00a803b579ff44f4351f6")
	hashes = append(hashes, s)

	// Request some Headers
	messageID := uuid.New().String()
	ch := h2.AddMessageToPending(messageID)
	rpc2.RequestHeaders(messageID, hashes)
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
	rpc2.RequestBlocks(messageID, hashes)
	<-ch
	data = h2.GetDataFromMessage(messageID)
	for _, msg := range data {
		resp := msg.(*protobuf.BlocksResponse)
		for _, protoBlock := range resp.BlockData {
			block := libnet.ProtoBlockToBlock(protoBlock)
			log.Print(block.Header.PreviousHash.String())
		}
	}

	_ = h1 // silence unused warning
}