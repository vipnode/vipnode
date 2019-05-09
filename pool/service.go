package pool

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/internal/pretty"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool/balance"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

// defaultRequestNumHosts is the default number of hosts to
// request as a client, if not provided explicitly.
const defaultRequestNumHosts = 3

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

	Store           store.Store
	BalanceManager  balance.Manager
	ClientMessager  func(nodeID string) string
	MaxRequestHosts int               // MaxRequestHosts is the maximum number of hosts a client is allowed to request (0 is unlimited)
	RestrictNetwork ethnode.NetworkID // TODO: Wire this up

	skipWhitelist bool // skipWhitelist is used for testing.

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
	inactive, err := p.Store.UpdateNodePeers(store.NodeID(nodeID), peers, req.BlockNumber)
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
		logger.Printf("Host update %q: %d peers, %d active, %d invalid. %s", pretty.Abbrev(nodeID), len(peers), len(validPeers), len(inactive), nodeBalance.String())
	} else {
		logger.Printf("Client update %q: %d peers, %d active, %d invalid. %s", pretty.Abbrev(nodeID), len(peers), len(validPeers), len(inactive), nodeBalance.String())
	}

	return &resp, nil
}

// Host registers a full node to participate as a vipnode host in this pool.
// DEPRECATED: Use Connect
func (p *VipnodePool) Host(ctx context.Context, sig string, nodeID string, nonce int64, req HostRequest) (*HostResponse, error) {
	// This is a backport of Host using Connect behind the scenes.
	if err := p.verify(sig, "vipnode_host", nodeID, nonce, req); err != nil {
		return nil, err
	}

	connectReq := ConnectRequest{
		NodeInfo: ethnode.UserAgent{
			Kind:       ethnode.ParseNodeKind(req.Kind),
			IsFullNode: true,
		},
		Payout:  req.Payout,
		NodeURI: req.NodeURI,
	}
	connectResp, err := p.connect(ctx, nodeID, connectReq)
	if err != nil {
		return nil, err
	}

	resp := &HostResponse{
		PoolVersion: connectResp.PoolVersion,
	}
	return resp, nil
}

// Client returns a list of enodes who are ready for the client node to connect.
// DEPRECATED: Use Connect
func (p *VipnodePool) Client(ctx context.Context, sig string, nodeID string, nonce int64, req ClientRequest) (*ClientResponse, error) {
	// This is a backport of Client using Connect behind the scenes.
	if err := p.verify(sig, "vipnode_client", nodeID, nonce, req); err != nil {
		return nil, err
	}
	// Clients have a default number of hosts they request. Hosts don't.
	numRequestHosts := defaultRequestNumHosts
	if req.NumHosts > 0 {
		numRequestHosts = req.NumHosts
	}
	connectReq := ConnectRequest{
		NumHosts: numRequestHosts,
		NodeInfo: ethnode.UserAgent{
			Kind:       ethnode.ParseNodeKind(req.Kind),
			IsFullNode: false,
		},
	}
	connectResp, err := p.connect(ctx, nodeID, connectReq)
	if err != nil {
		return nil, err
	}
	resp := &ClientResponse{
		Hosts:       connectResp.Hosts,
		PoolVersion: connectResp.PoolVersion,
		Message:     connectResp.Message,
	}
	return resp, nil
}

// Client returns a list of enodes who are ready for the client node to connect.
func (p *VipnodePool) Connect(ctx context.Context, sig string, nodeID string, nonce int64, req ConnectRequest) (*ConnectResponse, error) {
	if err := p.verify(sig, "vipnode_connect", nodeID, nonce, req); err != nil {
		return nil, err
	}

	return p.connect(ctx, nodeID, req)
}

// connect is same as Connect without signature verification. Used as a helper.
// TODO: We can inline connect into Connect once Client/Host are removed.
func (p *VipnodePool) connect(ctx context.Context, nodeID string, req ConnectRequest) (*ConnectResponse, error) {
	kind := req.NodeInfo.Kind.String()
	if kind == "unknown" {
		kind = ""
	}

	isHost := req.NodeInfo.IsFullNode
	if p.RestrictNetwork != 0 && p.RestrictNetwork != req.NodeInfo.Network {
		return nil, fmt.Errorf("node is on the wrong network, pool requires: %s", p.RestrictNetwork)
	}

	response := &ConnectResponse{
		PoolVersion: p.Version,
	}
	if p.ClientMessager != nil {
		response.Message = p.ClientMessager(nodeID)
	}

	// FIXME: Need to investigate if there's a vuln here where a client could
	// get successfully whitelisted, then switched to host status thus bypass
	// billing?
	node := store.Node{
		ID:       store.NodeID(nodeID),
		Kind:     kind,
		LastSeen: time.Now(),
		IsHost:   isHost,
		Payout:   store.Account(req.Payout),
	}

	if isHost {
		// We only care about publicly-visible nodeURIs for hosts.
		service, err := jsonrpc2.CtxService(ctx)
		if err != nil {
			return nil, err
		}

		remoteHost := ""
		if withAddr, ok := service.(interface{ RemoteAddr() string }); ok {
			remoteHost = (&url.URL{Host: withAddr.RemoteAddr()}).Hostname()
		}
		defaultPort := "30303"
		node.URI, err = normalizeNodeURI(req.NodeURI, nodeID, remoteHost, defaultPort)
		if err != nil {
			return nil, err
		}

		// TODO: Clean up disconnected/failed host services
		p.mu.Lock()
		p.remoteHosts[node.ID] = service
		p.mu.Unlock()
	}

	if err := p.Store.SetNode(node); err != nil {
		return nil, err
	}

	if err := p.BalanceManager.OnClient(node); err != nil {
		return nil, err
	}

	reqKind := kind
	if isHost {
		// Any kind of host peer will do.
		reqKind = ""
	}

	hosts, err := p.requestHosts(ctx, nodeID, req.NumHosts, reqKind)
	if err != nil {
		return nil, err
	}

	if !isHost && len(hosts) == 0 {
		logger.Printf("New %q peer: %q (no active hosts found)", kind, pretty.Abbrev(nodeID))
		return nil, NoHostNodesError{}
	}

	response.Hosts = hosts
	if p.skipWhitelist {
		logger.Printf("New %q peer: %q (%d hosts found, skipping whitelist)", kind, pretty.Abbrev(nodeID), len(hosts))
	} else {
		logger.Printf("New %q peer: %q (%d hosts found)", kind, pretty.Abbrev(nodeID), len(hosts))
	}

	return response, nil
}

func (p *VipnodePool) requestHosts(ctx context.Context, nodeID string, numRequestHosts int, kind string) ([]store.Node, error) {
	if p.MaxRequestHosts > 0 && numRequestHosts > p.MaxRequestHosts {
		numRequestHosts = p.MaxRequestHosts
	}

	var hosts []store.Node
	if numRequestHosts == 0 {
		// Nothing left to do
		return hosts, nil
	}

	r, err := p.Store.ActiveHosts(kind, numRequestHosts)
	if err != nil {
		return nil, err
	}

	if p.skipWhitelist {
		// Bypass whitelisting, used for making testing simpler
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
		err = RemoteHostErrors{"vipnode_whitelist", errors}
	}

	logger.Printf("Request hosts from %q: kind=%q (%d hosts found, %d accepted), failures=%s", pretty.Abbrev(nodeID), kind, len(remotes), len(accepted), err)

	if len(accepted) >= 1 {
		// We're okay returning without an error as long as some hosts succeeded.
		return accepted, nil
	}

	if err != nil {
		return nil, err
	}

	return nil, NoHostNodesError{len(r)}
}

// Ping returns "pong", used for testing.
func (p *VipnodePool) Ping(ctx context.Context) string {
	return "pong"
}
