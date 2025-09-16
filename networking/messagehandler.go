package networking

import (
	"fmt"
	"net"
)

// ProcessMessage handles different types of network messages
func (network *Network) ProcessMessage(msg *Message, peer *Peer, conn net.Conn) {
	// First check if this is a response to a pending request
	if handled := network.HandleResponse(msg); handled {
		fmt.Printf("Delivered response for request %s\n", msg.ReplyTo)
		return
	}

	switch msg.Type {
	case MessageTypeHandshake:
		network.ProcessHandshake(msg, peer, conn)
	case MessageTypeNewBlockHeader:
		network.ProcessNewBlockHeader(msg, peer)
	case MessageTypeNewBlock:
		network.ProcessNewBlock(msg, peer)
	case MessageTypePing:
		network.SendPongResponse(conn)
	case MessageTypePong:
		network.ProcessPong(peer)
	case MessageTypeRequestPeers:
		network.SendPeerList(conn, msg, peer.Address)
	case MessageTypeSharePeers:
		network.ProcessSharedPeers(msg, peer)
	case MessageTypeRequestBlock:
		network.SendBlockResponse(conn, msg, peer)
	case MessageTypeNewTx:
		network.ProcessNewTransaction(msg, peer)
	case MessageTypeRequestChainHead:
		network.SendChainHeadResponse(conn, msg, peer)
	case MessageTypeChainHead:
		network.ProcessChainHead(msg, peer)
	case MessageTypeRequestHeaders:
		network.SendHeadersResponse(conn, msg, peer)
	case MessageTypeHeaders:
		network.ProcessHeaders(msg, peer)
	case MessageTypeRequestBlocks:
		network.SendBlocksResponse(conn, msg, peer)
	case MessageTypeBlocks:
		network.ProcessBlocks(msg, peer)
	case MessageTypeSubmitBlock:
		network.ProcessSubmitBlock(msg, conn)
	default:
		fmt.Printf("Unknown message type: %s\n", msg.Type)
	}
}
