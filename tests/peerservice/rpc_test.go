package main

import (
	"august/blockchain"
	"august/libnet"
	"august/libnet/protobuf"
	"august/libnet/rpc"
	"context"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/peer"
	"testing"
	"time"
)

func createTestPeer(t *testing.T, port int, seeds ...peer.AddrInfo) (*libnet.PeerService, *rpc.RPCProtocol) {
	var rpcProto *rpc.RPCProtocol
	ps, err := libnet.NewPeerService("127.0.0.1", port, context.Background(), func(ps *libnet.PeerService) {
		rpcProto = rpc.NewRPCProtocol(ps)
	}, seeds...)
	if err != nil {
		t.Fatalf("Failed to create peer service: %v", err)
	}
	return ps, rpcProto
}

func TestRPCHeaderRequest(t *testing.T) {
	ps1, rpc1 := createTestPeer(t, 9000)
	defer ps1.Host.Close()

	peerInfo1 := peer.AddrInfo{
		ID:    ps1.Host.ID(),
		Addrs: ps1.Host.Addrs(),
	}

	ps2, rpc2 := createTestPeer(t, 9001, peerInfo1)
	defer ps2.Host.Close()

	if err := ps2.Host.Connect(context.Background(), peerInfo1); err != nil {
		t.Fatalf("Failed to connect peers: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	var testHeader blockchain.BlockHeader
	testHeader.Height = 1
	testHash := testHeader.GetHash()

	receivedHeader := make(chan *blockchain.BlockHeader, 1)
	rpc1.SetOnHeaderAnnouncementCallback(func(header *blockchain.BlockHeader) {
		receivedHeader <- header
	})

	rpc2.AnnounceHeader(&testHeader)

	select {
	case header := <-receivedHeader:
		if header.GetHash() != testHash {
			t.Fatalf("Received header hash mismatch: expected %s, got %s", testHash.String(), header.GetHash().String())
		}
		t.Logf("Successfully received header announcement: %s", header.GetHash().String())
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for header announcement")
	}
}

func TestRPCBlockRequest(t *testing.T) {
	ps1, _ := createTestPeer(t, 9010)
	defer ps1.Host.Close()

	peerInfo1 := peer.AddrInfo{
		ID:    ps1.Host.ID(),
		Addrs: ps1.Host.Addrs(),
	}

	ps2, rpc2 := createTestPeer(t, 9011, peerInfo1)
	defer ps2.Host.Close()

	if err := ps2.Host.Connect(context.Background(), peerInfo1); err != nil {
		t.Fatalf("Failed to connect peers: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	testBlock := blockchain.GenesisBlock
	testHash := testBlock.Header.GetHash()

	ps1.Node = &mockNode{blocks: map[blockchain.Hash32]*blockchain.Block{testHash: testBlock}}

	messageID := uuid.New().String()
	ch := ps2.AddMessageToPending(messageID)
	rpc2.RequestBlocks(messageID, []blockchain.Hash32{testHash})

	select {
	case <-ch:
		data := ps2.GetDataFromMessage(messageID)
		if len(data) == 0 {
			t.Fatal("No block data received")
		}
		resp := data[0].(*protobuf.BlocksResponse)
		if len(resp.BlockData) == 0 {
			t.Fatal("No blocks in response")
		}
		block := libnet.ProtoBlockToBlock(resp.BlockData[0])
		if block.Header.GetHash() != testHash {
			t.Fatalf("Block hash mismatch: expected %s, got %s", testHash.String(), block.Header.GetHash().String())
		}
		t.Logf("Successfully received block: %s", block.Header.GetHash().String())
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for block response")
	}
}

type mockNode struct {
	blocks map[blockchain.Hash32]*blockchain.Block
}

func (m *mockNode) GetBlocks(hashes []blockchain.Hash32) []*blockchain.Block {
	var blocks []*blockchain.Block
	for _, hash := range hashes {
		if block := m.blocks[hash]; block != nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func (m *mockNode) GetHeaders(hashes []blockchain.Hash32) []*blockchain.BlockHeader {
	var headers []*blockchain.BlockHeader
	for _, hash := range hashes {
		if block := m.blocks[hash]; block != nil {
			headers = append(headers, &block.Header)
		}
	}
	return headers
}

func TestMassHeaderPropagation(t *testing.T) {
	numPeers := 5
	var peers []*libnet.PeerService
	var rpcs []*rpc.RPCProtocol

	ps1, rpc1 := createTestPeer(t, 9100)
	defer ps1.Host.Close()
	peers = append(peers, ps1)
	rpcs = append(rpcs, rpc1)

	peerInfo1 := peer.AddrInfo{
		ID:    ps1.Host.ID(),
		Addrs: ps1.Host.Addrs(),
	}

	for i := 1; i < numPeers; i++ {
		ps, rpcProto := createTestPeer(t, 9100+i, peerInfo1)
		defer ps.Host.Close()
		peers = append(peers, ps)
		rpcs = append(rpcs, rpcProto)

		if err := ps.Host.Connect(context.Background(), peerInfo1); err != nil {
			t.Fatalf("Failed to connect peer %d: %v", i, err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	receivedCount := make([]int, numPeers)
	for i := range numPeers {
		idx := i
		rpcs[i].SetOnHeaderAnnouncementCallback(func(header *blockchain.BlockHeader) {
			receivedCount[idx]++
		})
	}

	numHeaders := 10
	for i := range numHeaders {
		header := blockchain.BlockHeader{Height: uint64(i + 1)}
		rpcs[0].AnnounceHeader(&header)
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)

	for i := 1; i < numPeers; i++ {
		if receivedCount[i] != numHeaders {
			t.Errorf("Peer %d received %d headers, expected %d", i, receivedCount[i], numHeaders)
		}
	}
	t.Logf("Successfully propagated %d headers to %d peers", numHeaders, numPeers-1)
}

func TestMassBlockPropagation(t *testing.T) {
	numPeers := 5
	var peers []*libnet.PeerService
	var rpcs []*rpc.RPCProtocol

	ps1, rpc1 := createTestPeer(t, 9200)
	defer ps1.Host.Close()
	peers = append(peers, ps1)
	rpcs = append(rpcs, rpc1)

	peerInfo1 := peer.AddrInfo{
		ID:    ps1.Host.ID(),
		Addrs: ps1.Host.Addrs(),
	}

	blocks := make(map[blockchain.Hash32]*blockchain.Block)
	blocks[blockchain.GenesisBlock.Header.GetHash()] = blockchain.GenesisBlock
	mockNode := &mockNode{blocks: blocks}
	ps1.Node = mockNode

	for i := 1; i < numPeers; i++ {
		ps, rpcProto := createTestPeer(t, 9200+i, peerInfo1)
		defer ps.Host.Close()
		ps.Node = mockNode
		peers = append(peers, ps)
		rpcs = append(rpcs, rpcProto)

		if err := ps.Host.Connect(context.Background(), peerInfo1); err != nil {
			t.Fatalf("Failed to connect peer %d: %v", i, err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	numRequests := 10
	successCount := 0

	for i := range numRequests {
		messageID := uuid.New().String()
		ch := peers[i%numPeers].AddMessageToPending(messageID)
		rpcs[i%numPeers].RequestBlocks(messageID, []blockchain.Hash32{blockchain.GenesisBlock.Header.GetHash()})

		select {
		case <-ch:
			data := peers[i%numPeers].GetDataFromMessage(messageID)
			if len(data) > 0 {
				resp := data[0].(*protobuf.BlocksResponse)
				if len(resp.BlockData) > 0 {
					successCount++
				}
			}
		case <-time.After(2 * time.Second):
			t.Logf("Request %d timed out", i)
		}
	}

	if successCount < numRequests/2 {
		t.Errorf("Only %d/%d block requests succeeded", successCount, numRequests)
	}
	t.Logf("Successfully completed %d/%d block requests across %d peers", successCount, numRequests, numPeers)
}