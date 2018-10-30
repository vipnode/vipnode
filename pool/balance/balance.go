package balance

import (
	"github.com/vipnode/vipnode/pool/store"
)

type Manager interface {
	OnUpdate(node store.Node, peers []store.Node) (store.Balance, error)
	// TODO: OnConnect, OnDisconnect, etc? OnConnect would be useful for time-based trials.
	// TODO: Support error type that forces a disconnect (eg. trial expired?)
}
