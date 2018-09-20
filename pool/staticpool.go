package pool

import (
	"context"
	"errors"

	"github.com/vipnode/vipnode/pool/store"
)

// Type assert for Pool implementation.
var _ Pool = &StaticPool{}

// StaticPool is a dummy implementation of a pool service that always returns
// from the same set of host nodes. It does not do any signature checking.
type StaticPool struct {
	Nodes []store.Node
}

func (s *StaticPool) AddNode(nodeURI string) error {
	// TODO: Parse ID etc?
	s.Nodes = append(s.Nodes, store.Node{
		URI: nodeURI,
	})
	return nil
}

func (s *StaticPool) Host(ctx context.Context, req HostRequest) (*HostResponse, error) {
	return &HostResponse{}, nil
}

func (s *StaticPool) Client(ctx context.Context, req ClientRequest) (*ClientResponse, error) {
	return &ClientResponse{Hosts: s.Nodes}, nil
}

func (s *StaticPool) Disconnect(ctx context.Context) error {
	return nil
}

func (s *StaticPool) Update(ctx context.Context, req UpdateRequest) (*UpdateResponse, error) {
	return &UpdateResponse{}, nil
}

func (s *StaticPool) Withdraw(ctx context.Context) error {
	return errors.New("not implemented")
}
