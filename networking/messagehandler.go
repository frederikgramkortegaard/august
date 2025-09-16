package networking

import (
	"log"
	"net"
)

// ProcessMessage handles different types of network messages
func (s *Server) ProcessMessage(msg *Message, peer *Peer, conn net.Conn) {
	// First check if this is a response to a pending request
	if handled := s.HandleResponse(msg); handled {
		log.Printf(s.config.NodeID+"\t"+"Delivered response for request %s", msg.ReplyTo)
		return
	}

	switch msg.Type {
	case MessageTypeHandshake:
		s.ProcessHandshake(msg, peer, conn)
	case MessageTypeNewBlockHeader:
		s.ProcessNewBlockHeader(msg, peer)
	case MessageTypeNewBlock:
		s.ProcessNewBlock(msg, peer)
	case MessageTypePing:
		s.SendPongResponse(conn)
	case MessageTypePong:
		s.ProcessPong(peer)
	case MessageTypeRequestPeers:
		s.SendPeerList(conn, msg, peer.Address)
	case MessageTypeSharePeers:
		s.ProcessSharedPeers(msg, peer)
	case MessageTypeRequestBlock:
		s.SendBlockResponse(conn, msg, peer)
	case MessageTypeNewTx:
		s.ProcessNewTransaction(msg, peer)
	case MessageTypeRequestChainHead:
		s.SendChainHeadResponse(conn, msg, peer)
	case MessageTypeChainHead:
		s.ProcessChainHead(msg, peer)
	case MessageTypeRequestHeaders:
		s.SendHeadersResponse(conn, msg, peer)
	case MessageTypeHeaders:
		s.ProcessHeaders(msg, peer)
	case MessageTypeRequestBlocks:
		s.SendBlocksResponse(conn, msg, peer)
	case MessageTypeBlocks:
		s.ProcessBlocks(msg, peer)
	case MessageTypeSubmitBlock:
		s.ProcessSubmitBlock(msg, conn)
	default:
		log.Printf(s.config.NodeID+"\t"+"Unknown message type: %s", msg.Type)
	}
}
