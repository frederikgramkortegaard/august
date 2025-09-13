package networking

import (
	"net"
)

// ProcessMessage handles different types of network messages
func ProcessMessage(server *Server, msg *Message, peer *Peer, conn net.Conn) {
	// First check if this is a response to a pending request
	if handled := server.reqRespClient.HandleResponse(msg); handled {
		server.logf("Delivered response for request %s", msg.ReplyTo)
		return
	}

	switch msg.Type {
	case MessageTypeHandshake:
		server.ProcessHandshake(msg, peer, conn)
	case MessageTypeNewBlockHeader:
		server.ProcessNewBlockHeader(msg, peer)
	case MessageTypeNewBlock:
		server.ProcessNewBlock(msg, peer)
	case MessageTypePing:
		server.SendPongResponse(conn)
	case MessageTypePong:
		server.ProcessPong(peer)
	case MessageTypeRequestPeers:
		server.SendPeerList(conn, msg, peer.Address)
	case MessageTypeSharePeers:
		server.ProcessSharedPeers(msg, peer)
	case MessageTypeRequestBlock:
		server.SendBlockResponse(conn, msg, peer)
	case MessageTypeNewTx:
		server.ProcessNewTransaction(msg, peer)
	case MessageTypeRequestChainHead:
		server.SendChainHeadResponse(conn, msg, peer)
	case MessageTypeChainHead:
		server.ProcessChainHead(msg, peer)
	case MessageTypeRequestHeaders:
		server.SendHeadersResponse(conn, msg, peer)
	case MessageTypeHeaders:
		server.ProcessHeaders(msg, peer)
	case MessageTypeRequestBlocks:
		server.SendBlocksResponse(conn, msg, peer)
	case MessageTypeBlocks:
		server.ProcessBlocks(msg, peer)
	default:
		server.logf("Unknown message type: %s", msg.Type)
	}
}
