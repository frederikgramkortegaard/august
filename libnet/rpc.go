package libnet

import (
	"august/blockchain"
	"august/config"
	"august/libnet/protobuf"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
	"log"
	"math/rand"
)

const (
	ServiceName = "august.rpc"
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

func samplePeers(peers []peer.ID, maxPeers int) []peer.ID {
	if maxPeers >= len(peers) {
		return peers
	}

	indices := rand.Perm(len(peers))
	subsample := make([]peer.ID, maxPeers)
	for i := range maxPeers {
		subsample[i] = peers[indices[i]]
	}
	return subsample

}

func (rpc *RPCProtocol) RequestHeaders(hashes []blockchain.Hash32) (string, <-chan error) {

	errorCh := make(chan error, 1)

	messageId := uuid.New().String()

	go func(messageId string) {
		defer close(errorCh)

		// @TODO : We want to actually do error handling here
		peers := rpc.Host.Host.Network().Peers()

		// Randomly subsample up to 'config.maxPeersForRequest' peers
		// and request headers for them all.
		// @TODO : Define a consensus mechanism for this, for now, we just
		// take the results from the first peer, this is very wrong

		// Convert to [][]byte
		hashBytes := make([][]byte, len(hashes))
		for i, h := range hashes {
			hashBytes[i] = h[:] // convert [32]byte to []byte
		}

		headerRequestData := &protobuf.HeadersRequest{
			MessageData: rpc.Host.NewMessageData(messageId),
			Hashes:      hashBytes,
		}

		// Create sample of Peers to send the request to
		maxPeers := config.MaxPeersForRequest
		subsample := make([]peer.ID, maxPeers)
		if maxPeers < len(peers) {
			indices := rand.Perm(len(peers))
			for i := range maxPeers {
				subsample[i] = peers[indices[i]]
			}
		} else {
			subsample = peers
		}

		// Setup pending request
		rpc.Host.PendingRequestsMu.Lock()
		rpc.Host.PendingRequests[headerRequestData.MessageData.MessageId] = make([]proto.Message, 1)
		rpc.Host.PendingRequestsMu.Unlock()

		// Send request
		for _, peer := range subsample {
			rpc.Host.sendProtoMessage(peer, RPCHeadersRequest, headerRequestData)
			log.Println("Sent RPCHeadersRequest to Peer", peer)
		}

		errorCh <- nil
	}(messageId)

	return messageId, errorCh

}


func (rpc *RPCProtocol) addMessagesToPending(messageID string) {}
func (rpc*RPCProtocol) notifyWaiters(data proto.Message, messageID string) {

	rpc.Host.PendingRequestsMu.Lock()
	rpc.Host.PendingRequests[messageID] = append(rpc.Host.PendingRequests[messageID], data)

	for _, ch := range rpc.Host.PendingRequestsWaiters[messageID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	rpc.Host.PendingRequestsWaiters[messageID] = nil
	rpc.Host.PendingRequestsMu.Unlock()
}

func (rpc *RPCProtocol) onHeadersRequest(s network.Stream) {
	defer s.Close()

	data := &protobuf.HeadersRequest{}
	if err := ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	log.Printf("%s; Received Headers Request from %s. Message: %+v",
		s.Conn().LocalPeer(),
		s.Conn().RemotePeer(),
		data.MessageData)

	for i, h := range data.Hashes {
		var hash [32]byte
		copy(hash[:], h)
		log.Printf("Hash %d: %x", i, hash)
	}

	messageID := data.MessageData.MessageId
	rpc.notifyWaiters(data, messageID)

}
func (rpc *RPCProtocol) onHeadersResponse(s network.Stream) {}
