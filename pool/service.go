package pool

import (
	"context"
	"errors"
	"time"
)

// ErrNoHostNodes is returned when the pool does not have any hosts available.
var ErrNoHostNodes = errors.New("no host nodes available")

// Assert Pool implementation
var _ Pool = &pool{}

// TODO: locks? or are we doing chan msg passing wrapping?
type pool struct {
	// Registered balances
	balances map[account]Balance

	// Connected nodes
	clientnodes map[nodeID]ClientNode
	poolnodes   map[nodeID]HostNode
}

// register associates a wallet account with a nodeID, and increments the account's credit.
func (p *pool) register(nodeID nodeID, account account, credit amount) error {
	balance, ok := p.balances[account]
	if !ok {
		balance = Balance{Account: account}
	}
	balance.Credit += credit
	// TODO: Check if nodeID is already registered to another balance, if so remove it.
	return nil
}

// Update submits a list of peers that the node is connected to, returning the current account balance.
func (p *pool) Update(ctx context.Context, sig string, nodeID string, timestamp time.Time, peers string) (*Balance, error) {
	return nil, errors.New("not implemented yet")
}

// Connect returns a list of enodes who are ready for the client node to connect.
func (p *pool) Connect(ctx context.Context, sig string, nodeID string, timestamp time.Time, kind string) ([]HostNode, error) {
	// TODO: Send a whitelist request, only return subset of nodes that responded in time.
	r := []HostNode{}

	// TODO: Filter by kind (geth vs parity?)
	// TODO: Do something other than random, such as by availability.
	// FIXME: lol implicitly random map iteration
	for _, n := range p.poolnodes {
		r = append(r, n)
	}

	if len(r) == 0 {
		return nil, ErrNoHostNodes
	}

	return r, nil
}

// Disconnect removes the node from the pool and stops accumulating respective balances.
func (p *pool) Disconnect(ctx context.Context, sig string, nodeID string, timestamp time.Time) error {
	// TODO: ...
	return nil
}

// Withdraw schedules a balance withdraw for a node
func (p *pool) Withdraw(ctx context.Context, sig string, nodeID string, timestamp time.Time) error {
	// TODO:
	return errors.New("not implemented yet")
}
