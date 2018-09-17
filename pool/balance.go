package pool

import "github.com/vipnode/vipnode/pool/store"

type BalanceManager interface {
	OnUpdate(node store.Node, peers []store.Node) (store.Balance, error)
}

type payPerInterval struct {
	Store store.Store
}

func (b *payPerInterval) OnUpdate(node store.Node, peers []store.Node) (store.Balance, error) {
	// XXX: ...
	return store.Balance{}, nil
}
