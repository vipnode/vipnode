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
	Store store.BalanceStore
	// Interval normalizes cost-per-update to this interval. Even if updates
	// come every second, minute, or 10 minutes, the CostPerInterval will get
	// normalized by Interval to be the same over time.
	Interval time.Duration
	// CreditPerInterval is the cost per interval that gets credited to the host (and debited from the client)
	CreditPerInterval big.Int
	// MinBalance, if set, is the minimum balance a node must have before it gets errored out.
	MinBalance *big.Int

	// now is used for testing to override time-based behaviour
	now func() time.Time
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

// OnClient is called when a client connects to the pool. If an error is
// returned, the client is disconnected with the error.
func (b *payPerInterval) OnClient(node store.Node) error {
	if b.MinBalance == nil {
		return nil
	}
	balance, err := b.Store.GetNodeBalance(node.ID)
	if err != nil {
		return err
	}
	// TODO: Write a test for this
	total := new(big.Int).Add(&balance.Credit, &balance.Deposit)
	if b.MinBalance.Cmp(total) > 0 {
		return LowBalanceError{
			CurrentBalance: total,
			MinBalance:     b.MinBalance,
		}
	}
	return nil
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

	// If this comparison is in the wrong place, it could make the pool
	// insolvent. On the other hand, if we compare too early, then the client
	// could get into a loop where it disconnects due to low balance, connects
	// successfully, repeat.
	if b.MinBalance != nil && b.MinBalance.Cmp(total) > 0 {
		return store.Balance{}, LowBalanceError{
			CurrentBalance: total,
			MinBalance:     b.MinBalance,
		}
	}

	if err := b.Store.AddNodeBalance(node.ID, new(big.Int).Neg(total)); err != nil {
		return store.Balance{}, err
	}
	balance, err := b.Store.GetNodeBalance(node.ID)
	if err != nil {
		return balance, err
	}

	return b.Store.GetNodeBalance(node.ID)
}
