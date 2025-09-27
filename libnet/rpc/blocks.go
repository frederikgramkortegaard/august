
package rpc

import (
	"august/blockchain"
	"august/config"
	"august/libnet"
	"august/libnet/protobuf"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
)

const RPCBlocksRequest = "/august/rpc/blocksreq/1.0.0"
const RPCBlocksResponse = "/august/rpc/blocksresp/1.0.0"


func (rpc *RPCProtocol) RequestBlocks(messageID string, hashes []blockchain.Hash32) {

	peers := rpc.Host.Host.Network().Peers()

	// Convert to [][]byte
	hashBytes := make([][]byte, len(hashes))
	for i, h := range hashes {
		hashBytes[i] = h[:] // convert [32]byte to []byte
	}

	blockRequestData := &protobuf.BlocksRequest{
		MessageData: rpc.Host.NewMessageData(messageID),
		Hashes:      hashBytes,
	}

	// Randomly subsample up to 'config.maxPeersForRequest' peers
	// and request headers for them all.
	maxPeers := config.MaxPeersForRequest
	subsample := libnet.SamplePeers(peers, maxPeers)

	for _, peer := range subsample {
		log.Println("Sending RPCBlocksRequest to Peer", peer)
		rpc.Host.SendProtoMessage(peer, RPCBlocksRequest, blockRequestData)
	}
}
func (rpc *RPCProtocol) onBlocksRequest(s network.Stream){

	defer s.Close()

	data := &protobuf.BlocksRequest{}
	if err := libnet.ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	log.Printf("Received Blocks Request from %s",
		s.Conn().RemotePeer())

	// Attempt to find the headers
	hash32Blocks := make([]blockchain.Hash32, len(data.Hashes))

	for i, h := range data.Hashes {
		var hash [32]byte
		copy(hash[:], h)
		hash32Blocks[i] = hash
	}

	blocks := rpc.Host.Node.GetBlocks(hash32Blocks)

	blocksData := make([]*protobuf.BlockData, len(blocks))

	for i, h := range blocks {
		blocksData[i] = libnet.BlockToProtoBlock(h)
	}

	blockResponseData := &protobuf.BlocksResponse{
		MessageData: rpc.Host.NewMessageData(data.MessageData.MessageID),
		BlockData:  blocksData,
	}

	id, _ := peer.Decode(data.MessageData.SenderID)
	log.Println("Sending RPCBlocksResponse to Peer", data.MessageData.SenderID)
	rpc.Host.SendProtoMessage(id, RPCBlocksResponse, blockResponseData)
}
func (rpc *RPCProtocol) onBlocksResponse(s network.Stream){
	defer s.Close()

	data := &protobuf.BlocksResponse{}
	if err := libnet.ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	rpc.Host.PendingRequestsMu.Lock()
	log.Printf("Received Blocks Response with message id %s", data.MessageData.MessageID)
	rpc.Host.PendingRequests[data.MessageData.MessageID] = append(rpc.Host.PendingRequests[data.MessageData.MessageID], data)
	rpc.Host.PendingRequestsMu.Unlock()
	rpc.Host.NotifyMessageWaiters(data.MessageData.MessageID)
}

