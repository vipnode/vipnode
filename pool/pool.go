package coordinator

import (
	"context"
	"errors"
	"time"
)

var errNoNodes = errors.New("no nodes available")

// FIXME: placeholder types, replace with go-ethereum types
type account string
type nodeID string
type amount int

// Balance describes a node's account balance on the pool.
type Balance struct {
	Account      account
	Credit       amount
	NextWithdraw time.Time
}

// ClientNode stores metadata for tracking VIPs
type ClientNode struct {
	*Balance

	lastSeen time.Time
	peers    map[nodeID]time.Time // Last seen
}

// PoolNode stores metadata requires for tracking full nodes.
type PoolNode struct {
	*Balance

	url      string
	lastSeen time.Time
	peers    map[nodeID]time.Time // Last seen

	inSync bool // TODO: Do we need a penalty if a full node wants to accept peers while not in sync?
}

// TODO: locks? or are we doing chan msg passing wrapping?
type pool struct {
	// Registered balances
	balances map[account]Balance

	// Connected nodes
	clientnodes map[nodeID]ClientNode
	poolnodes   map[nodeID]PoolNode
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

// Update submits a list of peers that the node is conneucted to, returning the current account balance.
func (p *pool) Update(ctx context.Context, sig string, nodeID nodeID, timestamp time.Time, peers string) (*Balance, error) {
	return nil, errors.New("not implemented yet")
}

// Connect returns a list of enodes who are ready for the client node to connect.
func (p *pool) Connect(ctx context.Context, sig string, nodeID nodeID, timestamp time.Time) ([]PoolNode, error) {
	// TODO: Send a whitelist request, only return subset of nodes that responded in time.
	r := []PoolNode{}

	// TODO: Do something other than random, such as by availability.
	// FIXME: lol implicitly random map iteration
	for _, n := range p.poolnodes {
		r = append(r, n)
	}

	if len(r) == 0 {
		return nil, errNoNodes
	}

	return r, nil
}

// Withdraw schedules a balance withdraw for a node
func (p *pool) Withdraw(ctx context.Context, sig string, nodeID nodeID, timestamp time.Time) error {
	// TODO:
	return errors.New("not implemented yet")
}
