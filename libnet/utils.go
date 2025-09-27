package libnet

import (
	"august/config"
	"time"
	"context"
	pb "august/libnet/protobuf"
	"github.com/libp2p/go-libp2p/core/protocol"
	"math/rand"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
	"log"
)

func (n *P2PHost) NotifyMessageWaiters(messageID string) {
	log.Println("Notifying Waiters of message: ", messageID)
	n.PendingRequestsMu.Lock()
	defer n.PendingRequestsMu.Unlock()

	// If there are no waiters, ignore
	if _, ok := n.PendingRequestsWaiters[messageID]; !ok {
		return
	}

	// Close the channel (ch is buffered so no need to select/default)
	for _, ch := range n.PendingRequestsWaiters[messageID] {
		close(ch)
	}

	n.PendingRequestsWaiters[messageID] = nil
}

func (n *P2PHost) GetDataFromMessage(messageID string) []proto.Message {
	n.PendingRequestsMu.Lock()
	defer n.PendingRequestsMu.Unlock()
	return n.PendingRequests[messageID]

}

func (n *P2PHost) AddMessageToPending(messageID string) <-chan struct{} {
	log.Println("Adding message to pending: ", messageID)
	n.PendingRequestsMu.Lock()
	defer n.PendingRequestsMu.Unlock()

	// Create the channel the waiter will be informed of arrival from
	ch := make(chan struct{}, 1)

	// Create the entry if it doesn't already exist (protect against overwriting
	if _, ok := n.PendingRequests[messageID]; !ok {
		n.PendingRequests[messageID] = make([]proto.Message, 0)
	}

	// If there are no other waiters for this message, we have to setup the initial structure
	if _, ok := n.PendingRequestsWaiters[messageID]; !ok {
		n.PendingRequestsWaiters[messageID] = make([]chan struct{}, 0)
	}
	// Add the waiting channel
	n.PendingRequestsWaiters[messageID] = append(n.PendingRequestsWaiters[messageID], ch)
	return ch
}

func SamplePeers(peers []peer.ID, maxSample int) []peer.ID {
	if len(peers) <= maxSample {
		return peers
	}
	subsample := make([]peer.ID, maxSample)
	indices := rand.Perm(len(peers))
	for i := 0; 0 <  maxSample; i++ {
		subsample[i] = peers[indices[i]]
	}
	return subsample

}

func (n *P2PHost) NewMessageData(messageID string) *pb.MessageData {
	return &pb.MessageData{ClientVersion: config.RPCClientVersion,
		Timestamp: uint64(time.Now().Unix()),
		MessageID: messageID,
		SenderID:  n.Host.ID().String(),
	}
}

func (n *P2PHost) SendProtoMessage(id peer.ID, p protocol.ID, data proto.Message) bool {
	s, err := n.Host.NewStream(context.Background(), id, p)
	if err != nil {
		log.Println("Failed to open stream:", err)
		return false
	}
	defer s.Close()

	WriteProtoMsgStream(s, data)
	return true
}

