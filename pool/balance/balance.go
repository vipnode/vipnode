package balance

import (
	"github.com/vipnode/vipnode/pool/store"
)

// Manager is the minimal interface required to support a payment scheme. The
// payment implementation will receive handler calls.
// TODO: OnConnect, OnDisconnect, etc? OnConnect would be useful for time-based trials.
// TODO: Support error type that forces a disconnect (eg. trial expired?)
type Manager interface {
	// OnUpdate is called every time the state of a node's peers is updated.
	OnUpdate(node store.Node, peers []store.Node) (store.Balance, error)
}
