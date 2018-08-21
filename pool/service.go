package pool

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

// ErrNoHostNodes is returned when the pool does not have any hosts available.
type ErrNoHostNodes struct {
	NumTried int
}

func (err ErrNoHostNodes) Error() string {
	if err.NumTried == 0 {
		return "no host nodes available"
	}
	return fmt.Sprintf("no available host nodes found after trying %d nodes", err.NumTried)
}

// ErrVerifyFailed is returned when a signature fails to verify. It embeds
// the underlying Cause.
type ErrVerifyFailed struct {
	Cause  error
	Method string
}

func (err ErrVerifyFailed) Error() string {
	return fmt.Sprintf("method %q failed to verify signature: %s", err.Method, err.Cause)
}

// ErrConnectFailed is returned when connect fails to
// whitelist the client on remote hosts.
type ErrConnectFailed struct {
	Errors []error
}

func (err ErrConnectFailed) Error() string {
	if len(err.Errors) == 0 {
		return "no host connection errors"
	}
	var s strings.Builder
	fmt.Fprintf(&s, "failed to connect to %d hosts: ", len(err.Errors))
	for i, e := range err.Errors {
		s.WriteString(e.Error())
		if i != len(err.Errors)-1 {
			s.WriteString("; ")
		}
	}
	return s.String()
}

type hostService struct {
	store.HostNode
	jsonrpc2.Service
}

// New returns a VipnodePool implementation of Pool with the default memory
// store, which includes balance tracking.
func New() *VipnodePool {
	return &VipnodePool{
		// TODO: Replace with persistent store
		Store:       store.MemoryStore(),
		remoteHosts: map[store.NodeID]jsonrpc2.Service{},
	}
}

// VipnodePool implements a Pool service with balance tracking.
type VipnodePool struct {
	Store         store.Store
	skipWhitelist bool

	mu          sync.Mutex
	remoteHosts map[store.NodeID]jsonrpc2.Service
}

func (p *VipnodePool) verify(sig string, method string, nodeID string, nonce int64, args ...interface{}) error {
	if err := p.Store.CheckAndSaveNonce(store.NodeID(nodeID), nonce); err != nil {
		return ErrVerifyFailed{Cause: err, Method: method}
	}

	if err := request.Verify(sig, method, nodeID, nonce, args...); err != nil {
		return ErrVerifyFailed{Cause: err, Method: method}
	}
	return nil
}

// register associates a wallet account with a nodeID, and increments the account's credit.
func (p *VipnodePool) register(nodeID store.NodeID, account store.Account, credit store.Amount) error {
	// TODO: Check if nodeID is already registered to another balance, if so remove it
	return p.Store.AddBalance(account, credit)
}

// Update submits a list of peers that the node is connected to, returning the current account balance.
func (p *VipnodePool) Update(ctx context.Context, sig string, nodeID string, nonce int64, peers []string) (*store.Balance, error) {
	if err := p.verify(sig, "vipnode_update", nodeID, nonce, peers); err != nil {
		return nil, err
	}

	// XXX: Track peers.
	// XXX: Return proper balance
	return &store.Balance{}, nil
}

// Host registers a full node to participate as a vipnode host in this pool.
func (p *VipnodePool) Host(ctx context.Context, sig string, nodeID string, nonce int64, kind string, payout string, nodeURI string) error {
	if err := p.verify(sig, "vipnode_host", nodeID, nonce, kind, payout, nodeURI); err != nil {
		return err
	}

	// Confirm that nodeURI matches nodeID
	uri, err := url.Parse(nodeURI)
	if err != nil {
		return err
	}

	if uri.User.Username() != nodeID {
		return fmt.Errorf("nodeID [%s...] does not match nodeURI: %s", nodeID[:8], nodeURI)
	}

	logger.Printf("New host: %s", nodeURI)

	node := store.HostNode{
		ID:       store.NodeID(nodeID),
		URI:      nodeURI,
		Kind:     kind,
		LastSeen: time.Now(),
	}
	err = p.Store.SetHostNode(node, store.Account(payout))
	if err != nil {
		return err
	}

	service, err := jsonrpc2.CtxService(ctx)
	if err != nil {
		return err
	}
	// FIXME: Clean up disconnected hosts
	p.mu.Lock()
	p.remoteHosts[node.ID] = service
	p.mu.Unlock()

	return nil
}

// Connect returns a list of enodes who are ready for the client node to connect.
func (p *VipnodePool) Connect(ctx context.Context, sig string, nodeID string, nonce int64, kind string) ([]store.HostNode, error) {
	// FIXME: Kind might be insufficient: We need to distinguish between full node vs parity LES and geth LES.
	// TODO: Send a whitelist request, only return subset of nodes that responded in time.
	if err := p.verify(sig, "vipnode_connect", nodeID, nonce, kind); err != nil {
		return nil, err
	}

	logger.Printf("New client: %s", nodeID)

	// TODO: Unhardcode 3?
	numRequestHosts := 3
	r := p.Store.GetHostNodes(kind, numRequestHosts)
	if len(r) == 0 {
		return nil, ErrNoHostNodes{}
	}

	if p.skipWhitelist {
		return r, nil
	}

	errors := []error{}
	remotes := make([]hostService, 0, len(r))
	p.mu.Lock()
	for _, node := range r {
		remote, ok := p.remoteHosts[node.ID]
		if ok {
			remotes = append(remotes, hostService{
				node, remote,
			})
		} else {
			errors = append(errors, fmt.Errorf("missing remote service for candidate host: %q", node.ID))
		}
	}
	p.mu.Unlock()

	// TODO: Set deadline
	// TODO: Parallelize
	accepted := make([]store.HostNode, 0, len(remotes))
	for _, remote := range remotes {
		if err := remote.Service.Call(ctx, nil, "vipnode_whitelist", nodeID); err != nil {
			errors = append(errors, err)
			continue
		}
		accepted = append(accepted, remote.HostNode)
	}

	if len(accepted) >= 1 {
		if len(errors) > 0 {
			logger.Printf("Connect succeeded despite errors: %s", ErrConnectFailed{errors})
		}
		return accepted, nil
	}

	if len(errors) > 0 {
		return nil, ErrConnectFailed{errors}
	}

	return nil, ErrNoHostNodes{len(r)}
}

// Disconnect removes the node from the pool and stops accumulating respective balances.
func (p *VipnodePool) Disconnect(ctx context.Context, sig string, nodeID string, nonce int64) error {
	if err := p.verify(sig, "vipnode_disconnect", nodeID, nonce); err != nil {
		return err
	}

	// TODO: ...
	return nil
}

// Withdraw schedules a balance withdraw for a node
func (p *VipnodePool) Withdraw(ctx context.Context, sig string, nodeID string, nonce int64) error {
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
