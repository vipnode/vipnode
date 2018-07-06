package pool

import (
	"context"
)

// Pool represents a vipnode pool for coordinating between clients and hosts.
type Pool interface {
	Connect(ctx context.Context, sig string, nodeID string, nonce int64, kind string) ([]HostNode, error)
	Disconnect(ctx context.Context, sig string, nodeID string, nonce int64) error
	Update(ctx context.Context, sig string, nodeID string, nonce int64, peers string) (*Balance, error)
	Withdraw(ctx context.Context, sig string, nodeID string, nonce int64) error
}
