package pool

import (
	"context"

	"github.com/vipnode/vipnode/pool/store"
)

// Pool represents a vipnode pool for coordinating between clients and hosts.
type Pool interface {
	Connect(ctx context.Context, sig string, nodeID string, nonce int64, kind string) ([]store.HostNode, error)
	Disconnect(ctx context.Context, sig string, nodeID string, nonce int64) error
	Update(ctx context.Context, sig string, nodeID string, nonce int64, peers []string) (*store.Balance, error)
	Withdraw(ctx context.Context, sig string, nodeID string, nonce int64) error
}
