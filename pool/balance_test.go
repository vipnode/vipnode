package pool

import (
	"math/big"
	"testing"
	"time"

	"github.com/vipnode/vipnode/pool/store"
)

func TestBalanceInterval(t *testing.T) {
	now := time.Now()
	balanceManager := &payPerInterval{
		Interval:          time.Minute * 1,
		CreditPerInterval: *big.NewInt(1000),
		now:               func() time.Time { return now },
	}

	amount := balanceManager.intervalCredit(now.Add(-time.Minute * 2))
	if got, want := amount.Int64(), int64(2000); want != got {
		t.Errorf("got: %d; want: %d", got, want)
	}
}

func TestBalanceManager(t *testing.T) {
	storeDriver := store.MemoryStore()

	now := time.Now()
	balanceManager := &payPerInterval{
		Store:             storeDriver,
		Interval:          time.Minute * 1,
		CreditPerInterval: *big.NewInt(1000),
		now:               func() time.Time { return now },
	}

	nodes := []store.Node{}
	{
		ids := []string{"a", "b"}
		for _, id := range ids {
			parsedID, err := store.ParseNodeID(id)
			if err != nil {
				t.Fatal(err)
			}
			node := store.Node{
				ID:       parsedID,
				LastSeen: now,
				IsHost:   id == "a",
			}
			nodes = append(nodes, node)
			if err := storeDriver.SetNode(node); err != nil {
				t.Fatal(err)
			}
		}
	}

	check := func(node store.Node, peers []store.Node, wantBalance int64) {
		t.Helper()
		balance, err := balanceManager.OnUpdate(node, peers)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := balance.Credit.Int64(), wantBalance; got != want {
			t.Errorf("[node=%s, host=%t] incorrect balance: got %d; want %d", node.ID, node.IsHost, got, want)
		}
	}

	check(nodes[1], nodes[0:1], 0)
	check(nodes[0], nodes[1:], 0) // host

	nodes[0].LastSeen = now
	nodes[1].LastSeen = now
	now = now.Add(time.Minute * 5)

	check(nodes[1], nodes[0:1], -5000)
	check(nodes[0], nodes[1:], 5000) // host

	nodes[0].LastSeen = now
	nodes[1].LastSeen = now
	now = now.Add(time.Minute * 2)

	check(nodes[1], nodes[0:1], -7000)
	check(nodes[0], nodes[1:], 7000) // host
}
