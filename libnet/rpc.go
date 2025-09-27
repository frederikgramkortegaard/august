package libnet

import (
	"august/blockchain"
	"august/config"
	"august/libnet/protobuf"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
)

const RPCHeadersRequest = "/august/rpc/headersreq/1.0.0"
const RPCHeadersResponse = "/august/rpc/headersresp/1.0.0"

type RPCProtocol struct {
	Host *P2PHost
}

func NewRPCProtocol(h *P2PHost) *RPCProtocol {
	rpc := RPCProtocol{h}
	h.Host.SetStreamHandler(RPCHeadersRequest, rpc.onHeadersRequest)
	h.Host.SetStreamHandler(RPCHeadersResponse, rpc.onHeadersResponse)
	return &rpc
}

func (rpc *RPCProtocol) onHeadersRequest(s network.Stream) {
	defer s.Close()

	data := &protobuf.HeadersRequest{}
	if err := ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	log.Printf("Received Headers Request from %s",
		s.Conn().RemotePeer())

	// Attempt to find the headers
	var hash32Headers []blockchain.Hash32

	for _, h := range data.Hashes {
		var hash [32]byte
		copy(hash[:], h)
		hash32Headers = append(hash32Headers, hash)
	}

	headers := rpc.Host.Node.GetHeaders(hash32Headers)
	log.Print("test", hash32Headers)
	log.Print("test", headers)

	var headersData []*protobuf.HeaderData

	for _, h := range headers {
		headersData = append(headersData, &protobuf.HeaderData{
			Version:      h.Version,
			PreviousHash: h.PreviousHash[:],
			Height:       h.Height,
			Timestamp:    h.Timestamp,
			Nonce:        h.Nonce,
			MerkleRoot:   h.MerkleRoot[:],
			StateRoot:    h.StateRoot[:],
			Bits:         h.Bits,
			TotalWork:    h.TotalWork,
			GasLimit:     h.GasLimit,
			GasUsed:      h.GasUsed,
			Beneficiary:  h.Beneficiary[:],
		})
	}

	headerResponseData := &protobuf.HeadersResponse{
		MessageData: rpc.Host.NewMessageData(data.MessageData.MessageID),
		HeaderData:  headersData,
	}

	id, _ := peer.Decode(data.MessageData.SenderID)
	log.Println("Sending RPCHeadersResponse to Peer", data.MessageData.SenderID)
	rpc.Host.sendProtoMessage(id, RPCHeadersResponse, headerResponseData)

}

func (rpc *RPCProtocol) RequestHeaders(messageID string, hashes []blockchain.Hash32) {

	peers := rpc.Host.Host.Network().Peers()

	// Convert to [][]byte
	hashBytes := make([][]byte, len(hashes))
	for i, h := range hashes {
		log.Print("vv", h)
		hashBytes[i] = h[:] // convert [32]byte to []byte
	}

	headerRequestData := &protobuf.HeadersRequest{
		MessageData: rpc.Host.NewMessageData(messageID),
		Hashes:      hashBytes,
	}

	// Randomly subsample up to 'config.maxPeersForRequest' peers
	// and request headers for them all.
	maxPeers := config.MaxPeersForRequest
	subsample := samplePeers(peers, maxPeers)

	for _, peer := range subsample {
		log.Println("Sending RPCHeadersRequest to Peer", peer)
		rpc.Host.sendProtoMessage(peer, RPCHeadersRequest, headerRequestData)
	}

}

func (rpc *RPCProtocol) onHeadersResponse(s network.Stream) {
	defer s.Close()

	data := &protobuf.HeadersResponse{}
	if err := ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	rpc.Host.PendingRequestsMu.Lock()
	log.Printf("Received Headers Response with message id %s", data.MessageData.MessageID)
	rpc.Host.PendingRequests[data.MessageData.MessageID] = append(rpc.Host.PendingRequests[data.MessageData.MessageID], data)
	rpc.Host.PendingRequestsMu.Unlock()
	rpc.Host.NotifyMessageWaiters(data.MessageData.MessageID)

}

func ProtoHeaderToBlockHeader(h *protobuf.HeaderData) *blockchain.BlockHeader {
	var previousHash, merkleRoot, stateRoot, beneficiary [32]byte
	copy(previousHash[:], h.PreviousHash)
	copy(merkleRoot[:], h.MerkleRoot)
	copy(stateRoot[:], h.StateRoot)
	copy(beneficiary[:], h.Beneficiary)

	return &blockchain.BlockHeader{
		Version:      h.Version,
		PreviousHash: previousHash,
		Height:       h.Height,
		Timestamp:    h.Timestamp,
		Nonce:        h.Nonce,
		MerkleRoot:   merkleRoot,
		StateRoot:    stateRoot,
		Bits:         h.Bits,
		TotalWork:    h.TotalWork,
		GasLimit:     h.GasLimit,
		GasUsed:      h.GasUsed,
		Beneficiary:  beneficiary,
	}
}

