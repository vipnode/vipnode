package pool

import (
	"context"
	"errors"
)

// Type assert for Pool implementation.
var _ Pool = &StaticPool{}

// StaticPool is a dummy implementation of a pool service that always returns
// from the same set of host nodes. It does not do any signature checking.
type StaticPool struct {
	Nodes []HostNode
}

func (s *StaticPool) Connect(ctx context.Context, sig string, nodeID string, nonce int64, kind string) ([]HostNode, error) {
	return s.Nodes, nil
}

func (s *StaticPool) Disconnect(ctx context.Context, sig string, nodeID string, nonce int64) error {
	return nil
}

func (s *StaticPool) Update(ctx context.Context, sig string, nodeID string, nonce int64, peers string) (*Balance, error) {
	return &Balance{}, nil
}

func (s *StaticPool) Withdraw(ctx context.Context, sig string, nodeID string, nonce int64) error {
	return errors.New("not implemented")
}
