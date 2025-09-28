// Package libnet defines an interface between libp2p and our utilization of it, e.g. custom protocols
package libnet

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"google.golang.org/protobuf/proto"
	"august/blockchain"
	"log"
	"sync"
)


type NodeInterface interface {
	GetBlocks(hashes []blockchain.Hash32) []*blockchain.Block
	GetHeaders(hashes []blockchain.Hash32) []*blockchain.BlockHeader
}

// RPCProtocol defines the interface for RPC protocol operations
// This avoids import cycles by defining the interface in libnet
type RPCProtocol interface {
	RequestHeaders(messageID string, hashes []blockchain.Hash32)
	RequestBlocks(messageID string, hashes []blockchain.Hash32)
	AnnounceHeader(header *blockchain.BlockHeader)
	AnnounceTransaction(tx *blockchain.Transaction)
	SetOnHeaderAnnouncementCallback(f func(*blockchain.BlockHeader))
	SetOnTransactionAnnouncementCallback(f func(*blockchain.Transaction))
}

type PeerService struct {
	Node      NodeInterface
	Host      host.Host
	DHT       *dht.IpfsDHT
	Discovery discovery.Discovery

	// RPC protocol handler (set via callback to avoid import cycles)
	RPC RPCProtocol

	// ReqResp
	PendingRequestsMu      sync.Mutex
	PendingRequests        map[string][]proto.Message
	PendingRequestsWaiters map[string][]chan struct{}
}

type ProtocolInitializer func(*PeerService)

// NewPeerService returns a pointer to a new PeerService structure
// It also initializes a Kademlia DHT and builds a RoutingDiscovery,
// on which it bootstraps by attempting to connect to 'seeds'
// The initProtocols callback is called after setup to allow registering protocol handlers
func NewPeerService(listenIP string, listenPort int, ctx context.Context, initProtocols ProtocolInitializer, seeds ...peer.AddrInfo) (*PeerService, error) {

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

	pn := &PeerService{
		Host:                   h,
		DHT:                    kad,
		Discovery:              routedDiscovery,
		PendingRequests:        make(map[string][]proto.Message, 0),
		PendingRequestsWaiters: make(map[string][]chan struct{}, 0),
	}

	// Initialize protocols if callback provided
	if initProtocols != nil {
		initProtocols(pn)
	}

	return pn, nil

}

func (n *PeerService) Close() error {
	if err := n.DHT.Close(); err != nil {
		return err
	}
	return n.Host.Close()
}
