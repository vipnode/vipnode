package pool

import (
	"fmt"
	"time"

	"github.com/vipnode/vipnode/pool/store"
)

type BalanceManager interface {
	OnUpdate(node store.Node, peers []store.Node) (store.Balance, error)
	// TODO: OnConnect, OnDisconnect, etc? OnConnect would be useful for time-based trials.
	// TODO: Support error type that forces a disconnect (eg. trial expired?)
}

type payPerInterval struct {
	Store             store.Store
	Interval          time.Duration
	CreditPerInterval store.Amount
}

// OnUpdate takes a node instance (with a Lastseen timestamp of the previous
// update) and the current active peers.
func (b *payPerInterval) OnUpdate(node store.Node, peers []store.Node) (store.Balance, error) {
	if node.IsHost {
		// We ignore host updates, only update balance on client updates. If
		// client fails to update, then the host will disconnect.
		return b.Store.GetBalance(node.ID)
	}
	if b.Interval <= 0 {
		// FIXME: Ideally this should be caught earlier. Maybe move to an earlier On* callback once we have more. Also check to make sure the values are big enough for the int64/float64 math.
		return store.Balance{}, fmt.Errorf("payPerInterval: Invalid interval value: %d", b.Interval)
	}
	delta := time.Now().Sub(node.LastSeen).Seconds()
	var total store.Amount
	for _, peer := range peers {
		credit := store.Amount((delta * float64(b.CreditPerInterval)) / b.Interval.Seconds())
		b.Store.AddBalance(peer.ID, credit)
		total += credit
	}
	if err := b.Store.AddBalance(node.ID, -total); err != nil {
		return store.Balance{}, err
	}
	return b.Store.GetBalance(node.ID)
}
