package pool

import (
	"context"

	"github.com/vipnode/vipnode/pool/store"
)

// Pool represents a vipnode pool for coordinating between clients and hosts.
type Pool interface {
	Connect(ctx context.Context, kind string) ([]store.HostNode, error)
	Disconnect(ctx context.Context) error
	Update(ctx context.Context, peers []string) (*store.Balance, error)
	Withdraw(ctx context.Context) error
}
