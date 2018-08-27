package client

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

// ErrAlreadyConnected is returned on Connect() if the client is already connected.
var ErrAlreadyConnected = errors.New("client already connected")

// Client represents a vipnode client which connects to a vipnode host.
type Client struct {
	ethnode.EthNode
	pool.Pool

	BalanceCallback *func(store.Balance)

	mu             sync.Mutex
	connectedHosts []store.Node
	disconnectChan chan struct{}
}

// Connect retrieves compatible hosts from the pool and connects to them.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.connectedHosts) > 0 {
		return ErrAlreadyConnected
	}
	c.disconnectChan = make(chan struct{}, 1)

	logger.Printf("Requesting host candidates...")
	ctx := context.TODO()
	kind := c.EthNode.Kind().String()
	nodes, err := c.Pool.Connect(ctx, kind)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return pool.ErrNoHostNodes{}
	}
	for _, node := range nodes {
		if err := c.EthNode.ConnectPeer(ctx, node.URI); err != nil {
			return err
		}
	}
	c.connectedHosts = nodes
	logger.Printf("Received %d host candidates, connecting.", len(nodes))
	return nil
}

func (c *Client) sendUpdate() error {
	ctx := context.TODO()
	peers, err := c.EthNode.Peers(ctx)
	if err != nil {
		return err
	}
	peerIDs := make([]string, 0, len(peers))
	for _, p := range peers {
		peerIDs = append(peerIDs, p.ID)
	}

	update, err := c.Pool.Update(ctx, peerIDs)
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

// ServeUpdates starts serving peering updates to the pool until Disconnect is
// called.
func (c *Client) ServeUpdates() error {
	if err := c.sendUpdate(); err != nil {
		return err
	}

	for {
		select {
		case <-time.After(store.KeepaliveInterval):
			if err := c.sendUpdate(); err != nil {
				return err
			}
		case <-c.disconnectChan:
			return nil
		}
	}
}

// Disconnect from hosts, also stop serving updates.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, node := range c.connectedHosts {
		if err := c.EthNode.DisconnectPeer(context.TODO(), node.URI); err != nil {
			return err
		}
	}
	c.connectedHosts = nil
	return nil
}
