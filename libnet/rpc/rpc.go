package rpc

import (
	"august/blockchain"
	"august/libnet"
)

type RPCProtocol struct {
	Host                         *libnet.PeerService
	onHeaderAnnouncementCallback func(*blockchain.BlockHeader)
}

func (rpc *RPCProtocol) setOnHeaderAnnouncementCallback(f func(*blockchain.BlockHeader)) {
	rpc.onHeaderAnnouncementCallback = f
}

func NewRPCProtocol(h *libnet.PeerService) *RPCProtocol {
	rpc := RPCProtocol{h, nil}
	h.Host.SetStreamHandler(RPCHeadersRequest, rpc.onHeadersRequest)
	h.Host.SetStreamHandler(RPCHeadersResponse, rpc.onHeadersResponse)
	h.Host.SetStreamHandler(RPCHeaderAnnouncement, rpc.onHeaderAnnouncement)
	h.Host.SetStreamHandler(RPCBlocksRequest, rpc.onBlocksRequest)
	h.Host.SetStreamHandler(RPCBlocksResponse, rpc.onBlocksResponse)
	return &rpc
}
