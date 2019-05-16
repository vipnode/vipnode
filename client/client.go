package client

import (
	"context"
	"errors"
	"time"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

const defaultNumHosts = 3

// ErrAlreadyConnected is returned on Connect() if the client is already connected.
var ErrAlreadyConnected = errors.New("client already connected")

func New(node ethnode.EthNode) *Client {
	return &Client{
		Version:  "dev",
		EthNode:  node,
		NumHosts: defaultNumHosts,

		stopCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}
}

// FIXME: I think we can rid of Client altogether, and merge with Host which is almost a superset.

// Client represents a vipnode client which connects to a vipnode host.
type Client struct {
	ethnode.EthNode

	// Version is the vipnode agent version that the client is using.
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

	// NumHosts is the number of vipnode hosts the client should try to connect with.
	// TODO: Autorequest more hosts if the number drops below this.
	NumHosts int

	stopCh chan struct{}
	waitCh chan error
}

// Wait blocks until the client is stopped.
func (c *Client) Wait() error {
	return <-c.waitCh
}

// Start retrieves compatible hosts from the pool and connects to them. Start
// blocks until registration is complete, then the keepalive peering updates
// break out into a separate goroutine and Start returns.
func (c *Client) Start(p pool.Pool) error {
	logger.Printf("Connecting to pool...")

	// We override IsFullNode here just because Client does not bother to
	// expose a reverse RPC service which the Connect RPC expects for hosts to
	// be able to whitelist. This will be moot when we merge Client+Host.
	nodeInfo := c.EthNode.UserAgent()
	nodeInfo.IsFullNode = false

	starCtx := context.Background()
	connResp, err := p.Connect(starCtx, pool.ConnectRequest{
		VipnodeVersion: c.Version,
		NodeInfo:       nodeInfo,
	})
	if err != nil {
		return err
	}
	logger.Printf("Connected to pool (version %s), updating state...", connResp.PoolVersion)

	if connResp.Message != "" && c.PoolMessageCallback != nil {
		c.PoolMessageCallback(connResp.Message)
	}

	if err := c.updatePeers(context.Background(), p); err != nil {
		return err
	}

	go func() {
		c.waitCh <- c.serveUpdates(p)
	}()

	return nil
}

func (c *Client) serveUpdates(p pool.Pool) error {
	ticker := time.Tick(store.KeepaliveInterval)
	for {
		select {
		case <-ticker:
			if err := c.updatePeers(context.Background(), p); err != nil {
				// FIXME: Does it make sense to continue updating for certain
				// errors? Eg if no hosts are found, we could keep sending
				// updates until we find some.
				return err
			}
		case <-c.stopCh:
			closeCtx := context.Background()
			// FIXME: Should we only disconnect from vipnode hosts?
			peers, err := c.EthNode.Peers(closeCtx)
			if err != nil {
				return err
			}
			for _, node := range peers {
				if err := c.EthNode.DisconnectPeer(closeCtx, node.ID); err != nil {
					return err
				}
			}
			return nil
		}
	}
}

func (c *Client) updatePeers(ctx context.Context, p pool.Pool) error {
	peers, err := c.EthNode.Peers(ctx)
	if err != nil {
		return err
	}

	// Do we need more peers?
	// FIXME: Does it make sense to request more peers before sending a vipnode_update?
	if needMore := c.NumHosts - len(peers); needMore > 0 {
		if err := c.addPeers(ctx, p, needMore); err != nil {
			return err
		}
	}

	update, err := p.Update(ctx, pool.UpdateRequest{PeerInfo: peers})
	if err != nil {
		return err
	}
	var balance store.Balance
	if c.BalanceCallback != nil && update.Balance != nil {
		balance = *update.Balance
		c.BalanceCallback(balance)
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

func (c *Client) addPeers(ctx context.Context, p pool.Pool, num int) error {
	logger.Printf("Requesting %d more hosts from pool...", num)
	peerResp, err := p.Peer(ctx, pool.PeerRequest{
		Num: num,
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
		if err := c.EthNode.ConnectPeer(ctx, node.URI); err != nil {
			return err
		}
	}
	return nil
}

// Disconnect from hosts, also stop serving updates.
func (c *Client) Stop() {
	c.stopCh <- struct{}{}
}
