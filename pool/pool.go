package pool

import (
	"context"
	"time"
)

// Pool represents a vipnode pool for coordinating between clients and hosts.
type Pool interface {
	Connect(ctx context.Context, sig string, nodeID string, timestamp time.Time, kind string) ([]HostNode, error)
	Disconnect(ctx context.Context, sig string, nodeID string, timestamp time.Time) error
	Update(ctx context.Context, sig string, nodeID string, timestamp time.Time, peers string) (*Balance, error)
	Withdraw(ctx context.Context, sig string, nodeID string, timestamp time.Time) error
}
