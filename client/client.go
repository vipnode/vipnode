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

	BalanceCallback *func(store.Balance)

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
	resp, err := p.Connect(starCtx, pool.ConnectRequest{Kind: kind})
	if err != nil {
		return err
	}
	nodes := resp.Hosts
	if len(nodes) == 0 {
		return pool.ErrNoHostNodes{}
	}
	logger.Printf("Received %d host candidates, connecting...", len(nodes))
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
		(*c.BalanceCallback)(*update.Balance)
	}

	var credit store.Amount
	if update.Balance != nil {
		credit = update.Balance.Credit
	}

	if len(update.InvalidPeers) > 0 {
		// Client doesn't really need to do anything if the pool stopped
		// tracking their host. That means the client is getting a free ride
		// and it's up to the host to kick the client when the host deems
		// necessary.
		logger.Printf("Update: %d peers connected, %d expired in pool, %d balance with pool.", len(peerIDs), len(update.InvalidPeers), credit)
	} else {
		logger.Printf("Update: %d peers connected, %d balance with pool.", len(peerIDs), credit)
	}

	return nil
}

// Disconnect from hosts, also stop serving updates.
func (c *Client) Stop() {
	c.stopCh <- struct{}{}
}
