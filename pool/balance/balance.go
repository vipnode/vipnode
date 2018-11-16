package balance

import (
	"fmt"
	"math/big"

	"github.com/vipnode/vipnode/pool/store"
)

// LowBalanceError is returned when the account's positive balance check fails.
type LowBalanceError struct {
	MinBalance     *big.Int
	CurrentBalance *big.Int
}

func (err LowBalanceError) Error() string {
	return fmt.Sprintf("low balance error: Current balance (%d) is less than the required minimum (%d)", err.CurrentBalance, err.MinBalance)
}

// Manager is the minimal interface required to support a payment scheme. The
// payment implementation will receive handler calls.
// TODO: OnConnect, OnDisconnect, etc? OnConnect would be useful for time-based trials.
// TODO: Support error type that forces a disconnect (eg. trial expired?)
type Manager interface {
	// OnClient is called when a client connects to the pool. If an error is
	// returned, the client is disconnected with the error.
	OnClient(node store.Node) error
	// OnUpdate is called every time the state of a node's peers is updated.
	OnUpdate(node store.Node, peers []store.Node) (store.Balance, error)
}
