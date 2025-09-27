package rpc

import (
	"august/blockchain"
	"august/libnet"
)

type RPCProtocol struct {
	Host                              *libnet.PeerService
	onHeaderAnnouncementCallback      func(*blockchain.BlockHeader)
	onTransactionAnnouncementCallback func(*blockchain.Transaction)
}

func (rpc *RPCProtocol) setOnHeaderAnnouncementCallback(f func(*blockchain.BlockHeader)) {
	rpc.onHeaderAnnouncementCallback = f
}

func (rpc *RPCProtocol) setOnTransactionAnnouncementCallback(f func(*blockchain.Transaction)) {
	rpc.onTransactionAnnouncementCallback = f
}

func NewRPCProtocol(h *libnet.PeerService) *RPCProtocol {
	rpc := RPCProtocol{h, nil, nil}
	h.Host.SetStreamHandler(RPCHeadersRequest, rpc.onHeadersRequest)
	h.Host.SetStreamHandler(RPCHeadersResponse, rpc.onHeadersResponse)
	h.Host.SetStreamHandler(RPCHeaderAnnouncement, rpc.onHeaderAnnouncement)
	h.Host.SetStreamHandler(RPCBlocksRequest, rpc.onBlocksRequest)
	h.Host.SetStreamHandler(RPCBlocksResponse, rpc.onBlocksResponse)
	return &rpc
}
