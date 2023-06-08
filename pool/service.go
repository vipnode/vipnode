package pool

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/vipnode/vipnode/v2/ethnode"
	"github.com/vipnode/vipnode/v2/internal/pretty"
	"github.com/vipnode/vipnode/v2/jsonrpc2"
	"github.com/vipnode/vipnode/v2/pool/balance"
	"github.com/vipnode/vipnode/v2/pool/store"
	"github.com/vipnode/vipnode/v2/request"
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
		Version: "dev",

		Store:            storeDriver,
		BalanceManager:   manager,
		remoteHosts:      map[store.NodeID]jsonrpc2.Service{},
		remoteNodeLookup: map[jsonrpc2.Service]store.NodeID{},
	}
}

const poolWhitelistTimeout = 5 * time.Second

// VipnodePool implements a Pool service with balance tracking.
type VipnodePool struct {
	// Version is returned as the PoolVersion in the ClientResponse when a new client connects.
	Version string

	Store               store.Store
	BalanceManager      balance.Manager
	ClientMessager      func(nodeID string) string
	MaxRequestHosts     int                                     // MaxRequestHosts is the maximum number of hosts a client is allowed to request (0 is unlimited)
	RestrictNetwork     ethnode.NetworkID                       // TODO: Wire this up
	BlockNumberProvider func(ethnode.NetworkID) (uint64, error) // BlockNumberProvider returns the latest block number that is known for the given network.
	CheckPeer           func(ethnode.PeerInfo) bool             // CheckPeer is a callback that can return false to mark a peer as invalid

	skipWhitelist bool // skipWhitelist is used for testing.

	mu               sync.Mutex
	remoteHosts      map[store.NodeID]jsonrpc2.Service
	remoteNodeLookup map[jsonrpc2.Service]store.NodeID // Reverse lookup
}

// TODO: Move CloseRemote and NumRemotes, and remoteHosts etc into a separate struct?

// CloseRemote is to be called when a remote service is disconnected. It is used to clean up state.
func (p *VipnodePool) CloseRemote(remote jsonrpc2.Service) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	nodeID, ok := p.remoteNodeLookup[remote]
	if !ok {
		// Nothing to clean up
		return nil
	}

	delete(p.remoteNodeLookup, remote)
	delete(p.remoteHosts, nodeID)

	return nil
}

// NumRemotes returns the number of remote hosts that the pool is currently maintaining.
func (p *VipnodePool) NumRemotes() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.remoteHosts)
}

func (p *VipnodePool) verify(sig string, method string, nodeID string, nonce int64, args ...interface{}) error {
	// TODO: Switch nonce to strictly timestamp within X time
	// TODO: Switch NodeID to pubkey?
	if err := request.Verify(sig, method, nodeID, nonce, args...); err != nil {
		return VerifyFailedError{Cause: err, Method: method}
	}

	// We check and save the nonce only after the verify passed, this allows us
	// to check twice for backwards compatibility.
	if err := p.Store.CheckAndSaveNonce(nodeID, nonce); err != nil {
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
		// Try again with old version (DEPRECATED)
		if errOld := p.verify(sig, "vipnode_update", nodeID, nonce, oldUpdateRequest{req.Peers, req.BlockNumber}); errOld != nil {
			return nil, err
		}
	}

	node, err := p.Store.GetNode(store.NodeID(nodeID))
	if err != nil {
		return nil, err
	}
	nodeBeforeUpdate := *node

	peerIDs := ethnode.Peers(req.PeerInfo).IDs()
	count := len(req.PeerInfo)

	inactive, err := p.Store.UpdateNodePeers(store.NodeID(nodeID), peerIDs, req.BlockNumber)
	if err != nil {
		return nil, err
	}
	active, err := p.Store.NodePeers(store.NodeID(nodeID))
	if err != nil {
		return nil, err
	}

	resp := UpdateResponse{
		InvalidPeers: make([]string, 0, len(inactive)),
		ActivePeers:  make([]string, 0, len(active)),
	}
	for _, peerID := range inactive {
		resp.InvalidPeers = append(resp.InvalidPeers, string(peerID))
	}
	for _, peerNode := range active {
		resp.ActivePeers = append(resp.ActivePeers, peerNode.URI)
	}
	if p.CheckPeer != nil {
		for _, peer := range req.PeerInfo {
			if !p.CheckPeer(peer) {
				resp.InvalidPeers = append(resp.InvalidPeers, peer.EnodeID())
			}
		}
	}

	if p.BlockNumberProvider != nil {
		resp.LatestBlockNumber, err = p.BlockNumberProvider(p.RestrictNetwork)
		if err != nil {
			return nil, err
		}
	}

	nodeBalance, err := p.BalanceManager.OnUpdate(nodeBeforeUpdate, active)
	if err != nil {
		if _, ok := err.(balance.LowBalanceError); ok {
			disconnectErr := p.disconnectPeers(ctx, nodeID, active)
			if disconnectErr != nil {
				logger.Printf("Client disconnect due to low balance: %q; disconnect RPC errors: %s", pretty.Abbrev(nodeID), disconnectErr)
			} else {
				logger.Printf("Client disconnect due to low balance: %q", pretty.Abbrev(nodeID))
			}
		}
		return nil, err
	}
	resp.Balance = &nodeBalance

	nodeKind := node.Kind + "-light"
	if node.IsHost {
		nodeKind = node.Kind + "-full"
	}

	logger.Printf("Updated %s: peers=%d active=%d invalid=%d block=%d node=%s balance=%s", pretty.Abbrev(nodeID), count, len(active), len(inactive), req.BlockNumber, nodeKind, nodeBalance.String())

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
	connectReq := ConnectRequest{
		NodeInfo: ethnode.UserAgent{
			Kind:       ethnode.ParseNodeKind(req.Kind),
			IsFullNode: false,
		},
	}
	connectResp, err := p.connect(ctx, nodeID, connectReq)
	if err != nil {
		return nil, err
	}

	// Clients have a default number of hosts they request. Hosts don't.
	numRequestHosts := defaultRequestNumHosts
	if req.NumHosts > 0 {
		numRequestHosts = req.NumHosts
	}
	hosts, err := p.requestHosts(ctx, nodeID, numRequestHosts, req.Kind)
	if err != nil {
		return nil, err
	}

	resp := &ClientResponse{
		Hosts:       hosts,
		PoolVersion: connectResp.PoolVersion,
		Message:     connectResp.Message,
	}
	return resp, nil
}

// Connect returns a list of enodes who are ready for the client node to connect.
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
		ID:             store.NodeID(nodeID),
		Kind:           kind,
		LastSeen:       time.Now(),
		IsHost:         isHost,
		Payout:         store.Account(req.Payout),
		NodeVersion:    req.NodeInfo.Version,
		VipnodeVersion: req.VipnodeVersion,
	}

	if isHost {
		// Hosts expose a reverse-RPC for vipnode_whitelist.
		service, err := jsonrpc2.CtxService(ctx)
		if err != nil {
			return nil, err
		}

		// We only care about publicly-visible nodeURIs for hosts.
		remoteHost := ""
		if withAddr, ok := service.(interface{ RemoteAddr() string }); ok {
			remoteHost = (&url.URL{Host: withAddr.RemoteAddr()}).Hostname()
		}
		defaultPort := "30303"
		node.URI, err = normalizeNodeURI(req.NodeURI, nodeID, remoteHost, defaultPort)
		if err != nil {
			return nil, err
		}

		p.mu.Lock()
		p.remoteHosts[node.ID] = service
		p.remoteNodeLookup[service] = node.ID
		p.mu.Unlock()
	}

	if err := p.Store.SetNode(node); err != nil {
		return nil, err
	}

	if err := p.BalanceManager.OnClient(node); err != nil {
		return nil, err
	}

	enode := node.URI
	if enode == "" {
		enode = "enode://" + nodeID + "@"
	}
	logger.Printf("Connected %s peer: %q", req.NodeInfo.KindType(), enode)

	return response, nil
}

// Peer returns a list of enodes who are ready for the node to connect.
func (p *VipnodePool) Peer(ctx context.Context, sig string, nodeID string, nonce int64, req PeerRequest) (*PeerResponse, error) {
	// TODO: Should we use protocol capability (eth, les, pip) instead of Kind?
	// It's hard to get self-reported protocol capability versions though (les/2 vs just les).
	hosts, err := p.requestHosts(ctx, nodeID, req.Num, req.Kind)
	if err != nil {
		return nil, err
	}

	// TODO: Move logs here.

	response := &PeerResponse{
		Peers: hosts,
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

	// Get existing peers that we can skip
	selfNodeID := store.NodeID(nodeID)
	peers, err := p.Store.NodePeers(selfNodeID)
	if err != nil {
		return nil, err
	}
	skipPeers := make(map[store.NodeID]struct{}, len(peers)+1)
	skipPeers[selfNodeID] = struct{}{}
	for _, peer := range peers {
		skipPeers[peer.ID] = struct{}{}
	}

	// Note that ActiveHosts returns hosts that have been active in the last
	// minute. They may not be connected anymore, so we're likely to get fewer
	// valid peers than number we want. That's okay, the agent can ask again
	// next cycle for more.
	r, err := p.Store.ActiveHosts(kind, numRequestHosts+len(skipPeers))
	if err != nil {
		return nil, err
	}

	if p.skipWhitelist {
		// Bypass whitelisting, used for making testing simpler
		return r, nil
	}

	remotes := make([]hostService, 0, len(r))
	p.mu.Lock()
	for _, node := range r {
		if _, skip := skipPeers[node.ID]; skip {
			// Skip peers we're already connected to, and ourself
			continue
		}

		remote, ok := p.remoteHosts[node.ID]
		if ok {
			remotes = append(remotes, hostService{
				node, remote,
			})
		} else {
			// TODO: Good time to mark the host as inactive? Or would that mess
			// with assumptions about some grace period of activity we
			// currently have? (Would also require a new store API)
			logger.Printf("ActiveHost candidate does not have a connected remote, skipping: %s", node.ID)
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

	errors := []error{}
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
		logger.Printf("Request kind=%q hosts: %q (%d hosts found, %d accepted); failures: %s", kind, pretty.Abbrev(nodeID), len(remotes), len(accepted), err)
	} else {
		logger.Printf("Request kind=%q hosts: %q (%d hosts found, %d accepted)", kind, pretty.Abbrev(nodeID), len(remotes), len(accepted))
	}

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
