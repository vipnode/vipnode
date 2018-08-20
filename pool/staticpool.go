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
	Nodes []store.HostNode
}

func (s *StaticPool) AddNode(nodeURI string) error {
	// TODO: Parse ID etc?
	s.Nodes = append(s.Nodes, store.HostNode{
		URI: nodeURI,
	})
	return nil
}

func (s *StaticPool) Host(ctx context.Context, kind string, payout string, nodeURI string) error {
	return nil
}

func (s *StaticPool) Connect(ctx context.Context, kind string) ([]store.HostNode, error) {
	return s.Nodes, nil
}

func (s *StaticPool) Disconnect(ctx context.Context) error {
	return nil
}

func (s *StaticPool) Update(ctx context.Context, peers []string) (*store.Balance, error) {
	return &store.Balance{}, nil
}

func (s *StaticPool) Withdraw(ctx context.Context) error {
	return errors.New("not implemented")
}
