package balance

import (
	"fmt"
	"math/big"
	"time"

	"github.com/vipnode/vipnode/pool/store"
)

// PayPerInterval creates a balance Manager which implements a pay-per-interval scheme.
func PayPerInterval(storeDriver store.BalanceStore, interval time.Duration, creditPerInterval *big.Int) *payPerInterval {
	return &payPerInterval{
		Store:             storeDriver,
		Interval:          interval,
		CreditPerInterval: *creditPerInterval,
	}
}

type payPerInterval struct {
	Store             store.BalanceStore
	Interval          time.Duration
	CreditPerInterval big.Int
	now               func() time.Time
}

func (b *payPerInterval) intervalCredit(lastSeen time.Time) *big.Int {
	if b.now == nil {
		b.now = time.Now
	}
	delta := big.NewInt(int64(b.now().Sub(lastSeen)))
	interval := big.NewInt(int64(b.Interval))
	credit := new(big.Int).Mul(delta, &b.CreditPerInterval)
	return credit.Div(credit, interval)
}

// OnUpdate takes a node instance (with a LastSeen timestamp of the previous
// update) and the current active peers.
func (b *payPerInterval) OnUpdate(node store.Node, peers []store.Node) (store.Balance, error) {
	if node.IsHost {
		// We ignore host updates, only update balance on client updates. If
		// client fails to update, then the host will disconnect.
		return b.Store.GetNodeBalance(node.ID)
	}
	if b.Interval <= 0 || b.CreditPerInterval.Cmp(new(big.Int)) == 0 {
		// FIXME: Ideally this should be caught earlier. Maybe move to an earlier On* callback once we have more. Also check to make sure the values are big enough for the int64/float64 math.
		return store.Balance{}, fmt.Errorf("payPerInterval: Invalid interval settings: %d per %s", &b.CreditPerInterval, b.Interval)
	}

	credit := b.intervalCredit(node.LastSeen)
	if credit.Cmp(new(big.Int)) == 0 {
		// No time passed?
		return b.Store.GetNodeBalance(node.ID)
	}

	total := new(big.Int)
	for _, peer := range peers {
		b.Store.AddNodeBalance(peer.ID, credit)
		total.Add(total, credit)
	}
	if err := b.Store.AddNodeBalance(node.ID, new(big.Int).Neg(total)); err != nil {
		return store.Balance{}, err
	}
	return b.Store.GetNodeBalance(node.ID)
}
