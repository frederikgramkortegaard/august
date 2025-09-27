package rpc

import (
	"august/blockchain"
	"august/libnet"
	"august/libnet/protobuf"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"log"
)

const RPCTransactionAnnouncement = "/august/rpc/headerannounce/1.0.0"

func (rpc *RPCProtocol) onTransactionAnnouncement(s network.Stream) {
	defer s.Close()

	if rpc.onTransactionAnnouncementCallback == nil {
		return
	}

	data := &protobuf.TransactionData{}
	if err := libnet.ReadProtoMsgStream(s, data); err != nil {
		log.Print(err)
	}

	log.Printf("Received Transaction Announcement from %s",
		s.Conn().RemotePeer())

	h := libnet.ProtoTransactionToTransaction(data)
	rpc.onTransactionAnnouncementCallback(&h)

}
func (rpc *RPCProtocol) AnnounceTransaction(tsx *blockchain.Transaction, exclude peer.ID) {
	peers := rpc.Host.Host.Network().Peers()
	h := libnet.TransactionToProtoTransaction(*tsx)
	for _, peer := range peers {
		if peer == exclude {
			continue
		}
		log.Println("Sending RPCTransactionAnnouncement to Peer", peer)
		rpc.Host.SendProtoMessage(peer, RPCTransactionAnnouncement, h)
	}
}
