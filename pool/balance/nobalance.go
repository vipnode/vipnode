package balance

import "github.com/vipnode/vipnode/pool/store"

// NoBalance always returns an empty balance
type NoBalance struct{}

func (b NoBalance) OnUpdate(node store.Node, peers []store.Node) (store.Balance, error) {
	return store.Balance{}, nil
}

func (b NoBalance) OnClient(node store.Node) error {
	return nil
}
