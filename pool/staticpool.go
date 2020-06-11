package pool

import (
	"context"
	"errors"

	"github.com/vipnode/vipnode/v2/ethnode"
	"github.com/vipnode/vipnode/v2/pool/store"
)

// Type assert for Pool implementation.
var _ Pool = &StaticPool{}

// StaticPool is a dummy implementation of a pool service that always returns
// from the same set of host nodes. It does not do any signature checking.
type StaticPool struct {
	nodeStore         map[store.NodeID]store.Node
	LatestBlockNumber uint64
}

func (s *StaticPool) AddNode(nodeURI string) error {
	if s.nodeStore == nil {
		s.nodeStore = map[store.NodeID]store.Node{}
	}
	u, err := ethnode.ParseNodeURI(nodeURI)
	if err != nil {
		return err
	}
	n := store.Node{
		ID:  store.NodeID(u.ID()),
		URI: nodeURI,
	}
	s.nodeStore[n.ID] = n
	return nil
}

func (s *StaticPool) nodes() []store.Node {
	r := make([]store.Node, 0, len(s.nodeStore))
	for _, node := range s.nodeStore {
		r = append(r, node)
	}
	return r
}

func (s *StaticPool) Host(ctx context.Context, req HostRequest) (*HostResponse, error) {
	return &HostResponse{}, nil
}

func (s *StaticPool) Client(ctx context.Context, req ClientRequest) (*ClientResponse, error) {
	return &ClientResponse{Hosts: s.nodes()}, nil
}

func (s *StaticPool) Connect(ctx context.Context, req ConnectRequest) (*ConnectResponse, error) {
	return &ConnectResponse{
		PoolVersion: "staticpool",
		Hosts:       s.nodes(),
	}, nil
}

func (s *StaticPool) Peer(ctx context.Context, req PeerRequest) (*PeerResponse, error) {
	return &PeerResponse{Peers: s.nodes()}, nil
}

func (s *StaticPool) Disconnect(ctx context.Context) error {
	return nil
}

func (s *StaticPool) Update(ctx context.Context, req UpdateRequest) (*UpdateResponse, error) {
	s.LatestBlockNumber = req.BlockNumber

	r := make([]string, 0, len(s.nodeStore))
	for _, n := range s.nodes() {
		r = append(r, n.URI)
	}

	return &UpdateResponse{
		ActivePeers:       r,
		LatestBlockNumber: s.LatestBlockNumber,
	}, nil
}

func (s *StaticPool) Withdraw(ctx context.Context) error {
	return errors.New("not implemented")
}
