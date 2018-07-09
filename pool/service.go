package pool

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/vipnode/vipnode/request"
)

// ErrNoHostNodes is returned when the pool does not have any hosts available.
var ErrNoHostNodes = errors.New("no host nodes available")

// ErrInvalidNonce is returned when a signed request contains an invalid nonce.d
var ErrInvalidNonce = errors.New("invalid nonce")

type ErrVerifyFailed struct {
	verifyErr error
}

func (err ErrVerifyFailed) Error() string {
	return fmt.Sprintf("failed to verify signature: %s", err.verifyErr)
}

func New() *VipnodePool {
	return &VipnodePool{
		balances:    map[account]Balance{},
		clientnodes: map[nodeID]ClientNode{},
		poolnodes:   map[nodeID]HostNode{},
		nonces:      map[string]int{},
	}
}

// Assert Pool implementation
var _ Pool = &VipnodePool{}

// TODO: locks? or are we doing chan msg passing wrapping?
type VipnodePool struct {
	mu sync.Mutex

	// Registered balances
	balances map[account]Balance

	// Connected nodes
	clientnodes map[nodeID]ClientNode
	poolnodes   map[nodeID]HostNode

	nonces map[string]int
}

func (p *VipnodePool) verify(sig string, method string, nodeID string, nonce int, args ...interface{}) error {
	p.mu.Lock()
	if p.nonces[nodeID] >= nonce {
		return ErrVerifyFailed{ErrInvalidNonce}
	}
	p.nonces[nodeID] = nonce
	p.mu.Unlock()

	if err := request.Verify(sig, method, nodeID, nonce, args...); err != nil {
		return ErrVerifyFailed{err}
	}
	return nil
}

// register associates a wallet account with a nodeID, and increments the account's credit.
func (p *VipnodePool) register(nodeID nodeID, account account, credit amount) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	balance, ok := p.balances[account]
	if !ok {
		balance = Balance{Account: account}
	}
	balance.Credit += credit
	// TODO: Check if nodeID is already registered to another balance, if so remove it.
	return nil
}

// Update submits a list of peers that the node is connected to, returning the current account balance.
func (p *VipnodePool) Update(ctx context.Context, sig string, nodeID string, nonce int, peers string) (*Balance, error) {
	if err := p.verify(sig, "vipnode_update", nodeID, nonce, peers); err != nil {
		return nil, err
	}

	return nil, errors.New("not implemented yet")
}

// Connect returns a list of enodes who are ready for the client node to connect.
func (p *VipnodePool) Connect(ctx context.Context, sig string, nodeID string, nonce int, kind string) ([]HostNode, error) {
	// TODO: Send a whitelist request, only return subset of nodes that responded in time.
	r := []HostNode{}

	if err := p.verify(sig, "vipnode_connect", nodeID, nonce, kind); err != nil {
		return nil, err
	}

	p.mu.Lock()
	// TODO: Filter by kind (geth vs parity?)
	// TODO: Do something other than random, such as by availability.
	// FIXME: lol implicitly random map iteration
	for _, n := range p.poolnodes {
		r = append(r, n)
	}
	p.mu.Unlock()

	if len(r) == 0 {
		return nil, ErrNoHostNodes
	}

	return r, nil
}

// Disconnect removes the node from the pool and stops accumulating respective balances.
func (p *VipnodePool) Disconnect(ctx context.Context, sig string, nodeID string, nonce int) error {
	if err := p.verify(sig, "vipnode_disconnect", nodeID, nonce); err != nil {
		return err
	}

	// TODO: ...
	return nil
}

// Withdraw schedules a balance withdraw for a node
func (p *VipnodePool) Withdraw(ctx context.Context, sig string, nodeID string, nonce int) error {
	if err := p.verify(sig, "vipnode_withdraw", nodeID, nonce); err != nil {
		return err
	}

	// TODO:
	return errors.New("not implemented yet")
}

// Ping returns "pong", used for testing.
func (p *VipnodePool) Ping(ctx context.Context) string {
	return "pong"
}
