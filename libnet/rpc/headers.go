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

const RPCHeadersRequest = "/august/rpc/headersreq/1.0.0"
const RPCHeadersResponse = "/august/rpc/headersresp/1.0.0"

/* HEADERS */

func (rpc *RPCProtocol) onHeadersRequest(s network.Stream) {
	defer s.Close()

	data := &protobuf.HeadersRequest{}
	if err := libnet.ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	log.Printf("Received Headers Request from %s",
		s.Conn().RemotePeer())

	// Attempt to find the headers
	hash32Headers := make([]blockchain.Hash32, len(data.Hashes))

	for i, h := range data.Hashes {
		var hash [32]byte
		copy(hash[:], h)
		hash32Headers[i] = hash
	}

	headers := rpc.Host.Node.GetHeaders(hash32Headers)

	headersData := make([]*protobuf.HeaderData, len(headers))

	for i, h := range headers {
		headersData[i] = libnet.BlockHeaderToProtoHeader(h)
	}

	headerResponseData := &protobuf.HeadersResponse{
		MessageData: rpc.Host.NewMessageData(data.MessageData.MessageID),
		HeaderData:  headersData,
	}

	id, _ := peer.Decode(data.MessageData.SenderID)
	log.Println("Sending RPCHeadersResponse to Peer", data.MessageData.SenderID)
	rpc.Host.SendProtoMessage(id, RPCHeadersResponse, headerResponseData)

}

func (rpc *RPCProtocol) RequestHeaders(messageID string, hashes []blockchain.Hash32) {

	peers := rpc.Host.Host.Network().Peers()

	// Convert to [][]byte
	hashBytes := make([][]byte, len(hashes))
	for i, h := range hashes {
		hashBytes[i] = h[:] // convert [32]byte to []byte
	}

	headerRequestData := &protobuf.HeadersRequest{
		MessageData: rpc.Host.NewMessageData(messageID),
		Hashes:      hashBytes,
	}

	// Randomly subsample up to 'config.maxPeersForRequest' peers
	// and request headers for them all.
	maxPeers := config.MaxPeersForRequest
	subsample := libnet.SamplePeers(peers, maxPeers)
	 

	for _, peer := range subsample {
		log.Println("Sending RPCHeadersRequest to Peer", peer)
		rpc.Host.SendProtoMessage(peer, RPCHeadersRequest, headerRequestData)
	}

}

func (rpc *RPCProtocol) onHeadersResponse(s network.Stream) {
	defer s.Close()

	data := &protobuf.HeadersResponse{}
	if err := libnet.ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	rpc.Host.PendingRequestsMu.Lock()
	log.Printf("Received Headers Response with message id %s", data.MessageData.MessageID)
	rpc.Host.PendingRequests[data.MessageData.MessageID] = append(rpc.Host.PendingRequests[data.MessageData.MessageID], data)
	rpc.Host.PendingRequestsMu.Unlock()
	rpc.Host.NotifyMessageWaiters(data.MessageData.MessageID)

}

