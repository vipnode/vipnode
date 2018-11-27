package pool

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/vipnode/vipnode/internal/pretty"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool/balance"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

type hostService struct {
	store.Node
	jsonrpc2.Service
}

// New returns a new VipnodePool RPC service with the given storage driver and
// balance manager. If manager is nil, then balance.NoBalance{} is used.
func New(storeDriver store.Store, manager balance.Manager) *VipnodePool {
	if manager == nil {
		manager = balance.NoBalance{}
	}
	return &VipnodePool{
		Store:          storeDriver,
		BalanceManager: manager,
		remoteHosts:    map[store.NodeID]jsonrpc2.Service{},
	}
}

const poolWhitelistTimeout = 5 * time.Second

// VipnodePool implements a Pool service with balance tracking.
type VipnodePool struct {
	// Version is returned as the PoolVersion in the ClientResponse when a new client connects.
	Version string

	Store          store.Store
	BalanceManager balance.Manager
	ClientMessager func(nodeID string) string

	// skipWhitelist is used for testing.
	skipWhitelist bool

	mu          sync.Mutex
	remoteHosts map[store.NodeID]jsonrpc2.Service
}

func (p *VipnodePool) verify(sig string, method string, nodeID string, nonce int64, args ...interface{}) error {
	// TODO: Switch nonce to strictly timestamp within X time
	// TODO: Switch NodeID to pubkey?
	if err := p.Store.CheckAndSaveNonce(nodeID, nonce); err != nil {
		return VerifyFailedError{Cause: err, Method: method}
	}

	if err := request.Verify(sig, method, nodeID, nonce, args...); err != nil {
		return VerifyFailedError{Cause: err, Method: method}
	}
	return nil
}

func (p *VipnodePool) disconnectPeers(ctx context.Context, nodeID string, peers []store.Node) error {
	callCtx, cancel := context.WithTimeout(ctx, poolWhitelistTimeout)
	defer cancel()
	errCh := make(chan error, 1)
	count := 0
	p.mu.Lock()
	for _, peer := range peers {
		if remote, ok := p.remoteHosts[peer.ID]; ok {
			count += 1
			go func() {
				errCh <- remote.Call(callCtx, nil, "vipnode_disconnect", nodeID)
			}()
		}
	}
	p.mu.Unlock()

	errors := []error{}
	for i := 0; i < count; i++ {
		err := <-errCh
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return RemoteHostErrors{
			Method: "vipnode_disconnect",
			Errors: errors,
		}
	}
	return nil
}

// Update submits a list of peers that the node is connected to, returning the current account balance.
func (p *VipnodePool) Update(ctx context.Context, sig string, nodeID string, nonce int64, req UpdateRequest) (*UpdateResponse, error) {
	// TODO: Send sync status?
	if err := p.verify(sig, "vipnode_update", nodeID, nonce, req); err != nil {
		return nil, err
	}

	node, err := p.Store.GetNode(store.NodeID(nodeID))
	if err != nil {
		return nil, err
	}
	nodeBeforeUpdate := *node

	peers := req.Peers
	inactive, err := p.Store.UpdateNodePeers(store.NodeID(nodeID), peers)
	if err != nil {
		return nil, err
	}

	resp := UpdateResponse{
		InvalidPeers: make([]string, 0, len(inactive)),
	}
	for _, peer := range inactive {
		resp.InvalidPeers = append(resp.InvalidPeers, string(peer))
	}
	validPeers, err := p.Store.NodePeers(store.NodeID(nodeID))
	if err != nil {
		return nil, err
	}

	// FIXME: Is there a bug here when a host is connected to another host?
	// TODO: Test InvalidPeers

	nodeBalance, err := p.BalanceManager.OnUpdate(nodeBeforeUpdate, validPeers)
	if err != nil {
		if _, ok := err.(balance.LowBalanceError); ok {
			disconnectErr := p.disconnectPeers(ctx, nodeID, validPeers)
			if disconnectErr != nil {
				logger.Printf("Client disconnect due to low balance: %q; disconnect RPC errors: %s", pretty.Abbrev(nodeID), disconnectErr)
			} else {
				logger.Printf("Client disconnect due to low balance: %q", pretty.Abbrev(nodeID))
			}
		}
		return nil, err
	}
	resp.Balance = &nodeBalance

	if node.IsHost {
		logger.Printf("Host update %q: %d peers, %d active, %d invalid. Balance: %d", pretty.Abbrev(nodeID), len(peers), len(validPeers), len(inactive), &nodeBalance.Credit)
	} else {
		logger.Printf("Client update %q: %d peers, %d active, %d invalid: Balance: %d", pretty.Abbrev(nodeID), len(peers), len(validPeers), len(inactive), &nodeBalance.Credit)

	}

	return &resp, nil
}

// Host registers a full node to participate as a vipnode host in this pool.
func (p *VipnodePool) Host(ctx context.Context, sig string, nodeID string, nonce int64, req HostRequest) (*HostResponse, error) {
	// TODO: Send capabilities?
	if err := p.verify(sig, "vipnode_host", nodeID, nonce, req); err != nil {
		return nil, err
	}

	service, err := jsonrpc2.CtxService(ctx)
	if err != nil {
		return nil, err
	}

	remoteHost := ""
	if withAddr, ok := service.(interface{ RemoteAddr() string }); ok {
		remoteHost = (&url.URL{Host: withAddr.RemoteAddr()}).Hostname()
	}
	defaultPort := "30303"
	nodeURI, err := normalizeNodeURI(req.NodeURI, nodeID, remoteHost, defaultPort)
	if err != nil {
		return nil, err
	}

	// TODO: Confirm that it's a full node, not a light node? Doesn't super matter since if i
	// XXX: Check versions

	logger.Printf("New %q host: %q", req.Kind, nodeURI)

	node := store.Node{
		ID:       store.NodeID(nodeID),
		URI:      nodeURI,
		Kind:     req.Kind,
		LastSeen: time.Now(),
		IsHost:   true,
		Payout:   store.Account(req.Payout),
	}
	err = p.Store.SetNode(node)
	if err != nil {
		return nil, err
	}

	// FIXME: Clean up disconnected hosts
	p.mu.Lock()
	p.remoteHosts[node.ID] = service
	p.mu.Unlock()

	resp := &HostResponse{
		PoolVersion: p.Version,
	}
	return resp, nil
}

// Client returns a list of enodes who are ready for the client node to connect.
func (p *VipnodePool) Client(ctx context.Context, sig string, nodeID string, nonce int64, req ClientRequest) (*ClientResponse, error) {
	if err := p.verify(sig, "vipnode_client", nodeID, nonce, req); err != nil {
		return nil, err
	}

	kind := req.Kind
	// TODO: Unhardcode this, maybe add to ClientRequest (but limit to some number)
	numRequestHosts := 3

	response := &ClientResponse{
		PoolVersion: p.Version,
	}
	if p.ClientMessager != nil {
		response.Message = p.ClientMessager(nodeID)
	}

	node := store.Node{
		ID:       store.NodeID(nodeID),
		Kind:     kind,
		LastSeen: time.Now(),
		IsHost:   false,
	}
	if err := p.Store.SetNode(node); err != nil {
		return nil, err
	}

	if err := p.BalanceManager.OnClient(node); err != nil {
		return nil, err
	}

	r, err := p.Store.ActiveHosts(kind, numRequestHosts)
	if err != nil {
		return nil, err
	}
	if len(r) == 0 {
		logger.Printf("New %q client: %q (no active hosts found)", kind, pretty.Abbrev(nodeID))
		return nil, NoHostNodesError{}
	}

	if p.skipWhitelist {
		logger.Printf("New %q client: %q (%d hosts found, skipping whitelist)", kind, pretty.Abbrev(nodeID), len(r))
		response.Hosts = r
		return response, nil
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

	accepted := make([]store.Node, 0, len(remotes))
	callCtx, cancel := context.WithTimeout(ctx, poolWhitelistTimeout)

	// Parallelize whitelist, return any hosts that respond within the timeout.
	errChan := make(chan error)
	acceptChan := make(chan store.Node)

	for _, remote := range remotes {
		go func(service jsonrpc2.Service, node store.Node) {
			if err := service.Call(callCtx, nil, "vipnode_whitelist", nodeID); err != nil {
				errChan <- err
			} else {
				acceptChan <- node
			}
		}(remote.Service, remote.Node)
	}

	for i := len(remotes); i > 0; i-- {
		select {
		case node := <-acceptChan:
			accepted = append(accepted, node)
		case err := <-errChan:
			errors = append(errors, err)
		}
	}
	cancel()
	// TODO: Penalize hosts that failed to respond within the deadline?

	if len(errors) > 0 {
		logger.Printf("New %q client: %s (%d hosts found, %d accepted) %s", kind, nodeID[:8], len(remotes), len(accepted), RemoteHostErrors{"vipnode_whitelist", errors})
	} else {
		logger.Printf("New %q client: %s (%d hosts found, %d accepted)", kind, nodeID[:8], len(remotes), len(accepted))
	}

	if len(accepted) >= 1 {
		response.Hosts = accepted
		return response, nil
	}

	if len(errors) > 0 {
		return nil, RemoteHostErrors{"vipnode_whitelist", errors}
	}

	return nil, NoHostNodesError{len(r)}
}

// Ping returns "pong", used for testing.
func (p *VipnodePool) Ping(ctx context.Context) string {
	return "pong"
}
