package agent

import (
	"context"
	"time"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

const defaultNumHosts = 3

var startTimeout = 10 * time.Second
var updateTimeout = 10 * time.Second

func New(node ethnode.EthNode) *Agent {
	return &Agent{
		EthNode: node,
		Version: "dev",
		stopCh:  make(chan struct{}),
		waitCh:  make(chan error, 1),
	}
}

type Agent struct {
	ethnode.EthNode

	// NodeURI can be used to set the enode:// connection string that
	// the pool should advertise to clients. Normally, the pool will
	// automatically deduce this string from the connection IP and nodeID, but
	// we can provide an override if there is a non-standard port or if the
	// node runs on a different IP from the vipnode agent.
	NodeURI string

	// Version is the version of the vipnode agent that the host is running.
	Version string

	// BalanceCallback is called whenever the client receives a balance update
	// from the pool. It can be used for displaying the current balance to the
	// client.
	BalanceCallback func(store.Balance)

	// PoolMessageCallback is called whenever the client receives a message
	// from the pool. This can be a welcome message including rules and
	// instructions for how to manage the client's balance. It should be
	// displayed to the client.
	PoolMessageCallback func(string)

	// NumHosts is the minimum number of vipnode hosts the
	// client should maintain connections with.
	NumHosts int

	// Payout is the address to register to associate pool credits towards.
	Payout string

	stopCh   chan struct{}
	waitCh   chan error
	nodeInfo ethnode.UserAgent // cached during Start
}

// Start registers the node on the given pool and starts sending peer updates
// every store.KeepaliveInterval. It returns after
// successfully registering with the pool.
func (a *Agent) Start(p pool.Pool) error {
	startCtx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()

	enode, err := a.EthNode.Enode(startCtx)
	if err != nil {
		return err
	}
	logger.Printf("Connected to local node: %s", enode)

	connectReq := pool.ConnectRequest{
		Payout:         a.Payout,
		NodeURI:        a.NodeURI,
		VipnodeVersion: a.Version,
		NodeInfo:       a.EthNode.UserAgent(),
	}
	a.nodeInfo = connectReq.NodeInfo
	resp, err := p.Connect(startCtx, connectReq)
	if err != nil {
		return err
	}
	logger.Printf("Registered on pool: Version %s", resp.PoolVersion)

	if resp.Message != "" && a.PoolMessageCallback != nil {
		a.PoolMessageCallback(resp.Message)
	}

	if err := a.updatePeers(startCtx, p); err != nil {
		return err
	}

	go func() {
		a.waitCh <- a.serveUpdates(p)
	}()
	return nil
}

// Stop shuts down all the active connections cleanly.
func (a *Agent) Stop() {
	a.stopCh <- struct{}{}
}

// Wait blocks until the agent is stopped. It returns any errors that occur
// during stopping.
func (a *Agent) Wait() error {
	return <-a.waitCh
}

func (a *Agent) serveUpdates(p pool.Pool) error {
	ticker := time.Tick(store.KeepaliveInterval)
	for {
		select {
		case <-ticker:
			// FIXME: Sametimes a.updatePeers could take a while, does it make
			// sense to keep ticking every interval regardless? Or should we
			// make sure there is an offset between calls? What if there's
			// overlap, do we want a mutex?
			if err := a.updatePeers(context.Background(), p); err != nil {
				// FIXME: Does it make sense to continue updating for certain
				// errors? Eg if no hosts are found, we could keep sending
				// updates until we find some.
				return err
			}
		case <-a.stopCh:
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

func (a *Agent) updatePeers(ctx context.Context, p pool.Pool) error {
	peers, err := a.EthNode.Peers(ctx)
	if err != nil {
		return err
	}

	// Do we need more peers?
	// FIXME: Does it make sense to request more peers before sending a vipnode_update?
	if needMore := a.NumHosts - len(peers); needMore > 0 {
		if err := a.addPeers(ctx, p, needMore); err != nil {
			return err
		}
	}

	update, err := p.Update(ctx, pool.UpdateRequest{PeerInfo: peers})
	if err != nil {
		return err
	}
	var balance store.Balance
	if a.BalanceCallback != nil && update.Balance != nil {
		balance = *update.Balance
		a.BalanceCallback(balance)
	}
	if len(update.InvalidPeers) > 0 {
		// Client doesn't really need to do anything if the pool stopped
		// tracking their host. That means the client is getting a free ride
		// and it's up to the host to kick the client when the host deems
		// necessary.
		logger.Printf("Sent update: %d peers connected, %d expired in pool. Pool response: %s", len(peers), len(update.InvalidPeers), balance.String())
	} else {
		logger.Printf("Sent update: %d peers connected. Pool response: %s", len(peers), balance.String())
	}

	return nil
}

func (a *Agent) addPeers(ctx context.Context, p pool.Pool, num int) error {
	logger.Printf("Requesting %d more %q hosts from pool...", a.EthNode.Kind(), num)
	peerResp, err := p.Peer(ctx, pool.PeerRequest{
		Num:  num,
		Kind: a.nodeInfo.Kind.String(),
	})
	if err != nil {
		return err
	}
	nodes := peerResp.Peers
	if len(nodes) == 0 {
		return pool.NoHostNodesError{}
	}
	logger.Printf("Received %d host candidates from pool, connecting...", len(nodes))
	for _, node := range nodes {
		if err := a.EthNode.ConnectPeer(ctx, node.URI); err != nil {
			return err
		}
	}
	return nil
}
