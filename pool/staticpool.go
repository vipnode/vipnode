package pool

import (
	"context"
	"errors"
	"time"
)

// StaticPool is a dummy implementation of a pool service that always returns from the same set of host nodes.
type StaticPool struct {
	Nodes []HostNode
}

func (s *StaticPool) Connect(ctx context.Context, sig string, nodeID nodeID, timestamp time.Time, kind string) ([]HostNode, error) {
	return s.Nodes, nil
}

func (s *StaticPool) Update(ctx context.Context, sig string, nodeID nodeID, timestamp time.Time, peers string) (*Balance, error) {
	return &Balance{}, nil
}

func (s *StaticPool) Withdraw(ctx context.Context, sig string, nodeID nodeID, timestamp time.Time) error {
	return errors.New("not implemented")
}
