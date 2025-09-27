package rpc

import (
	"august/libnet"
)

const RPCBlocksRequest = "/august/rpc/blocksreq/1.0.0"
const RPCBlocksResponse = "/august/rpc/blocksresp/1.0.0"

type RPCProtocol struct {
	Host *libnet.P2PHost
}

func NewRPCProtocol(h *libnet.P2PHost) *RPCProtocol {
	rpc := RPCProtocol{h}
	h.Host.SetStreamHandler(RPCHeadersRequest, rpc.onHeadersRequest)
	h.Host.SetStreamHandler(RPCHeadersResponse, rpc.onHeadersResponse)

	h.Host.SetStreamHandler(RPCBlocksRequest, rpc.onBlocksRequest)
	h.Host.SetStreamHandler(RPCBlocksResponse, rpc.onBlocksResponse)
	return &rpc
}

/* CONVERSION FUNCTIONS */
