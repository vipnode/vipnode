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
		readyCh: make(chan struct{}, 1),
	}
}

// Client represents a vipnode client which connects to a vipnode host.
type Client struct {
	ethnode.EthNode

	BalanceCallback *func(store.Balance)

	connectedHosts []store.Node
	stopCh         chan struct{}
	readyCh        chan struct{}
}

// Ready returns a channel that yields when the client is registered on the
// pool, after the first peers update is sent.
func (c *Client) Ready() <-chan struct{} {
	return c.readyCh
}

// Connect retrieves compatible hosts from the pool and connects to them.
func (c *Client) Start(p pool.Pool) error {
	logger.Printf("Requesting host candidates...")
	starCtx := context.Background()
	kind := c.EthNode.Kind().String()
	nodes, err := p.Connect(starCtx, kind)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return pool.ErrNoHostNodes{}
	}
	logger.Printf("Received %d host candidates, connecting...", len(nodes))
	for _, node := range nodes {
		if err := c.EthNode.ConnectPeer(starCtx, node.URI); err != nil {
			return err
		}
	}
	connectedHosts := nodes

	if err := c.updatePeers(context.Background(), p); err != nil {
		return err
	}

	c.readyCh <- struct{}{}

	for {
		select {
		case <-time.After(store.KeepaliveInterval):
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
	return nil
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

	update, err := p.Update(ctx, peerIDs)
	if err != nil {
		return err
	}
	if c.BalanceCallback != nil {
		(*c.BalanceCallback)(*update.Balance)
	}

	if len(update.InvalidPeers) > 0 {
		// Client doesn't really need to do anything if the pool stopped
		// tracking their host. That means the client is getting a free ride
		// and it's up to the host to kick the client when the host deems
		// necessary.
		logger.Printf("Update: %d peers connected, %d expired in pool, %d balance with pool.", len(peerIDs), len(update.InvalidPeers), update.Balance.Credit)
	} else {
		logger.Printf("Update: %d peers connected, %d balance with pool.", len(peerIDs), update.Balance.Credit)
	}

	return nil
}

// Disconnect from hosts, also stop serving updates.
func (c *Client) Stop() {
	c.stopCh <- struct{}{}
}
