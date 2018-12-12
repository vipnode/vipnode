package client

import (
	"context"
	"errors"
	"time"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

// ErrAlreadyConnected is returned on Connect() if the client is already connected.
var ErrAlreadyConnected = errors.New("client already connected")

func New(node ethnode.EthNode) *Client {
	return &Client{
		EthNode: node,
		stopCh:  make(chan struct{}),
		waitCh:  make(chan error, 1),
	}
}

// Client represents a vipnode client which connects to a vipnode host.
type Client struct {
	ethnode.EthNode

	// BalanceCallback is called whenever the client receives a balance update
	// from the pool. It can be used for displaying the current balance to the
	// client.
	BalanceCallback func(store.Balance)

	// PoolMessageCallback is called whenever the client receives a message
	// from the pool. This can be a welcome message including rules and
	// instructions for how to manage the client's balance. It should be
	// displayed to the client.
	PoolMessageCallback func(string)

	connectedHosts []store.Node
	stopCh         chan struct{}
	waitCh         chan error
}

// Wait blocks until the client is stopped.
func (c *Client) Wait() error {
	return <-c.waitCh
}

// Start retrieves compatible hosts from the pool and connects to them. Start
// blocks until registration is complete, then the keepalive peering updates
// break out into a separate goroutine and Start returns.
func (c *Client) Start(p pool.Pool) error {
	logger.Printf("Requesting host candidates...")
	starCtx := context.Background()
	kind := c.EthNode.Kind().String()
	resp, err := p.Client(starCtx, pool.ClientRequest{Kind: kind})
	if err != nil {
		return err
	}
	if resp.Message != "" && c.PoolMessageCallback != nil {
		c.PoolMessageCallback(resp.Message)
	}
	nodes := resp.Hosts
	if len(nodes) == 0 {
		return pool.NoHostNodesError{}
	}
	logger.Printf("Received %d host candidates from pool (version %s), connecting...", len(nodes), resp.PoolVersion)
	for _, node := range nodes {
		if err := c.EthNode.ConnectPeer(starCtx, node.URI); err != nil {
			return err
		}
	}
	if err := c.updatePeers(context.Background(), p); err != nil {
		return err
	}

	go func() {
		c.waitCh <- c.serveUpdates(p, nodes)
	}()

	return nil
}

func (c *Client) serveUpdates(p pool.Pool, connectedHosts []store.Node) error {
	ticker := time.Tick(store.KeepaliveInterval)
	for {
		select {
		case <-ticker:
			if err := c.updatePeers(context.Background(), p); err != nil {
				return err
			}
		case <-c.stopCh:
			closeCtx := context.Background()
			for _, node := range connectedHosts {
				if err := c.EthNode.DisconnectPeer(closeCtx, node.URI); err != nil {
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
	peerIDs := make([]string, 0, len(peers))
	for _, p := range peers {
		peerIDs = append(peerIDs, p.ID)
	}

	update, err := p.Update(ctx, pool.UpdateRequest{Peers: peerIDs})
	if err != nil {
		return err
	}
	if c.BalanceCallback != nil && update.Balance != nil {
		c.BalanceCallback(*update.Balance)
	}

	if len(update.InvalidPeers) > 0 {
		// Client doesn't really need to do anything if the pool stopped
		// tracking their host. That means the client is getting a free ride
		// and it's up to the host to kick the client when the host deems
		// necessary.
		logger.Printf("Sent update: %d peers connected, %d expired in pool. Pool response: %s", len(peerIDs), len(update.InvalidPeers), update.Balance.String())
	} else {
		logger.Printf("Sent update: %d peers connected. Pool response: %s", len(peerIDs), update.Balance.String())
	}

	return nil
}

// Disconnect from hosts, also stop serving updates.
func (c *Client) Stop() {
	c.stopCh <- struct{}{}
}
