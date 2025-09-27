// Package libnet defines an interface between libp2p and our utilization of it, e.g. custom protocols
package libnet

import (
	"august/node"
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"google.golang.org/protobuf/proto"
	"log"
	"sync"
)

type P2PHost struct {
	Node      *node.Node
	Host      host.Host
	DHT       *dht.IpfsDHT
	Discovery discovery.Discovery

	// Protocols
	Rpcprotocol *RPCProtocol

	// ReqResp
	PendingRequestsMu      sync.Mutex
	PendingRequests        map[string][]proto.Message
	PendingRequestsWaiters map[string][]chan struct{}
}




// NewP2PHost returns a pointer to a new P2PHost structure
// It also initializes a Kademlia DHT and builds a RoutingDiscovery,
// on which it bootstraps by attempting to connect to 'seeds'
func NewP2PHost(listenIP string, listenPort int, ctx context.Context, seeds ...peer.AddrInfo) (*P2PHost, error) {

	var opt = libp2p.ListenAddrStrings(
		fmt.Sprintf("/ip4/%s/tcp/%d", listenIP, listenPort),
	)

	// Create LibP2P Host
	h, err := libp2p.New(opt)
	if err != nil {
		log.Println("Failed to create new libp2p host", err)
		return nil, err
	}

	// Bootstrap DHT by connecting to seed nodes
	for _, peer := range seeds {
		if err := h.Connect(ctx, peer); err != nil {
			log.Print("Failed to connect to seed", peer.ID)
		}
	}

	// Setup Discovery and DHT
	kad, err := dht.New(ctx, h)
	if err != nil {
		log.Println("Failed to setup DHT", err)
		return nil, err
	}
	if err := kad.Bootstrap(ctx); err != nil {
		log.Println("Failed to bootstrap kad", err)
		return nil, err
	}

	routedDiscovery := routing.NewRoutingDiscovery(kad)

	pn := &P2PHost{
		Host:                   h,
		DHT:                    kad,
		Discovery:              routedDiscovery,
		PendingRequests:        make(map[string][]proto.Message, 0),
		PendingRequestsWaiters: make(map[string][]chan struct{}, 0),
	}

	pn.Rpcprotocol = NewRPCProtocol(pn)

	return pn, nil

}

func (n *P2PHost) Close() error {
	if err := n.DHT.Close(); err != nil {
		return err
	}
	return n.Host.Close()
}
