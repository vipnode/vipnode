package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

const defaultNumHosts = 3

var startTimeout = 10 * time.Second
var updateTimeout = 10 * time.Second

// ErrAlreadyStarted is returned when the agent service is started after it has
// already been started before.
var ErrAlreadyStarted = errors.New("agent already started")

// ErrNoPeers is returned when more peers are requested but none were returned
// from the pool.
var ErrNoPeers = errors.New("no peers available")

// Agent is a companion process for nodes that manages the node's communication
// with a Vipnode Pool.
type Agent struct {
	ethnode.EthNode

	// NodeURI can be used to override the enode:// connection string that
	// the pool should advertise to other peers. Normally, the pool will
	// automatically deduce this string from the connection IP and nodeID, but
	// we can provide an override if there is a non-standard port or if the
	// node runs on a different IP from the vipnode agent. (Optional)
	NodeURI string

	// Version is the version of the vipnode agent that the host is running.
	Version string

	// BalanceCallback is called whenever the client receives a balance update
	// from the pool. It can be used for displaying the current balance to the
	// client. (Optional)
	BalanceCallback func(store.Balance)

	// PoolMessageCallback is called whenever the client receives a message
	// from the pool. This can be a welcome message including rules and
	// instructions for how to manage the client's balance. It should be
	// displayed to the client. (Optional)
	PoolMessageCallback func(string)

	// BlockNumberCallback is called every update with the agent node's block
	// number and the latest block number that the pool knows about.
	BlockNumberCallback func(nodeBlockNumber uint64, poolBlockNumber uint64)

	// NumHosts is the minimum number of vipnode hosts the client should
	// maintain connections with. (Optional)
	NumHosts int

	// Payout is the address to register to associate pool credits towards.
	// (Optional)
	Payout string

	// UpdateInterval is the time between updates sent to the pool. If not set,
	// then store.KeepaliveInterval is used.
	UpdateInterval time.Duration

	initOnce sync.Once
	mu       sync.Mutex
	started  bool
	stopCh   chan struct{}
	waitCh   chan error
	nodeInfo ethnode.UserAgent // cached during Start
}

func (a *Agent) init() {
	a.initOnce.Do(func() {
		a.stopCh = make(chan struct{})
		a.waitCh = make(chan error, 1)
	})
}

// Start registers the node on the given pool and starts sending peer updates
// every store.KeepaliveInterval. It returns after
// successfully registering with the pool.
func (a *Agent) Start(p pool.Pool) error {
	a.init()

	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return ErrAlreadyStarted
	}
	a.mu.Unlock()

	startCtx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()

	enode, err := a.EthNode.Enode(startCtx)
	if err != nil {
		return err
	}
	ua := a.EthNode.UserAgent()
	logger.Printf("Connected to local %s node: %s", ua.KindType(), enode)

	version := a.Version
	if version == "" {
		version = "dev"
	}

	connectReq := pool.ConnectRequest{
		Payout:         a.Payout,
		NodeURI:        a.NodeURI,
		VipnodeVersion: version,
		NodeInfo:       ua,
	}
	a.nodeInfo = connectReq.NodeInfo
	resp, err := p.Connect(startCtx, connectReq)
	if err != nil {
		return AgentPoolError{err, "Failed during pool connect request"}
	}
	logger.Printf("Registered on pool: Version %s", resp.PoolVersion)

	if resp.Message != "" && a.PoolMessageCallback != nil {
		a.PoolMessageCallback(resp.Message)
	}

	if err := a.UpdatePeers(startCtx, p); err != nil {
		return err
	}

	go func() {
		a.waitCh <- a.serveUpdates(p)
	}()
	return nil
}

// Whitelist a peer for this node.
func (a *Agent) Whitelist(ctx context.Context, nodeID string) error {
	logger.Printf("Received whitelist request: %s", nodeID)
	return a.EthNode.AddTrustedPeer(ctx, nodeID)
}

// Stop shuts down all the active connections cleanly.
func (a *Agent) Stop() {
	a.init()
	a.stopCh <- struct{}{}
}

// Wait blocks until the agent is stopped. It returns any errors that occur
// during stopping.
func (a *Agent) Wait() error {
	a.init()
	return <-a.waitCh
}

func (a *Agent) serveUpdates(p pool.Pool) error {
	interval := a.UpdateInterval
	if interval == 0 {
		interval = store.KeepaliveInterval
	}

	ticker := time.Tick(interval)
	for {
		select {
		case <-ticker:
			if err := a.UpdatePeers(context.Background(), p); err != nil {
				return err
			}
		case <-a.stopCh:
			a.mu.Lock()
			a.started = false
			a.mu.Unlock()

			// FIXME: Does it make sense to call a.disconnectPeers(...) here?
			return nil
		}
	}
}

// disconnectPeers tells the node to disconnect from its peers.
// DEPRECATED: This method is unused. It might be useful at some point though?
// If not, remove later. Or maybe it should be changed to only disconnect from
// vipnode-tracked peers?
func (a *Agent) disconnectPeers(ctx context.Context) error {
	peers, err := a.EthNode.Peers(ctx)
	if err != nil {
		return err
	}
	for _, node := range peers {
		if err := a.EthNode.DisconnectPeer(ctx, node.ID); err != nil {
			return err
		}
	}
	return nil
}

// UpdatePeers sends a vipnode_update to the given pool. It will request more
// peers if necessary. Normally this is done automatically via Agent.Start()
// every configured interval.
func (a *Agent) UpdatePeers(ctx context.Context, p pool.Pool) error {
	peers, err := a.EthNode.Peers(ctx)
	if err != nil {
		return err
	}

	blockNumber, err := a.EthNode.BlockNumber(ctx)
	if err != nil {
		return err
	}

	update, err := p.Update(ctx, pool.UpdateRequest{
		PeerInfo:    peers,
		BlockNumber: blockNumber,
	})
	if err != nil {
		return AgentPoolError{err, "Failed during pool update request"}
	}
	var balance store.Balance
	if a.BalanceCallback != nil && update.Balance != nil {
		balance = *update.Balance
		a.BalanceCallback(balance)
	}

	logger.Printf("Pool update: peers=%d active=%d invalid=%d block=%d balance=%s", len(peers), len(update.ActivePeers), len(update.InvalidPeers), blockNumber, balance.String())

	// Do we need more peers?
	if needMore := a.NumHosts - len(update.ActivePeers); needMore > 0 {
		if err := a.AddPeers(ctx, p, needMore); err == ErrNoPeers {
			// We can live without more peers for now, will try again next time
		} else if err != nil {
			return err
		}
	}

	if a.BlockNumberCallback != nil {
		a.BlockNumberCallback(blockNumber, update.LatestBlockNumber)
	}

	var errors []error
	for _, peerID := range update.InvalidPeers {
		if err := a.EthNode.RemoveTrustedPeer(ctx, peerID); err != nil {
			errors = append(errors, err)
		}
		if err := a.EthNode.DisconnectPeer(ctx, peerID); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to disconnect from invalid peers: %q", errors)
	}

	return nil
}

// AddPeers requests num peers from the pool and connects the node to them.
func (a *Agent) AddPeers(ctx context.Context, p pool.Pool, num int) error {
	kind := a.nodeInfo.Kind.String()
	logger.Printf("Requesting more %q peers from pool: %d", kind, num)
	peerResp, err := p.Peer(ctx, pool.PeerRequest{
		Num:  num,
		Kind: kind,
	})
	if err != nil && jsonrpc2.IsErrorCode(err, jsonrpc2.ErrCodeInternal) && strings.HasPrefix(err.Error(), "no available") {
		return ErrNoPeers
	} else if err != nil {
		return AgentPoolError{err, "Failed during pool peer request"}
	}
	nodes := peerResp.Peers
	logger.Printf("Received %d host candidates from pool.", len(nodes))
	for _, node := range nodes {
		if err := a.EthNode.ConnectPeer(ctx, node.URI); err != nil {
			return err
		}
	}
	return nil
}

// Service is the set of RPC calls exposed by an agent.
type Service interface {
	Whitelist(ctx context.Context, nodeID string) error
}
