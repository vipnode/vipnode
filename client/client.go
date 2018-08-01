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

// updateInterval is how frequently we send updates to the pool.
const updateInterval = time.Second * 60

// Client represents a vipnode client which connects to a vipnode host.
type Client struct {
	ethnode.EthNode
	pool.Pool

	BalanceCallback *func(store.Balance)

	mu             sync.Mutex
	connectedHosts []store.HostNode
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

	ctx := context.TODO()
	kind := c.EthNode.Kind().String()
	nodes, err := c.Pool.Connect(ctx, kind)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return pool.ErrNoHostNodes
	}
	for _, node := range nodes {
		if err := c.EthNode.ConnectPeer(ctx, node.URI); err != nil {
			return err
		}
	}
	c.connectedHosts = nodes
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

	balance, err := c.Pool.Update(ctx, peerIDs)
	if err != nil {
		return err
	}
	if c.BalanceCallback != nil {
		(*c.BalanceCallback)(*balance)
	}

	logger.Printf("Update: %d peers connected, %d balance with pool.", len(peerIDs), balance.Credit)

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
		case <-time.After(updateInterval):
			if err := c.sendUpdate(); err != nil {
				return err
			}
		case <-c.disconnectChan:
			return nil
		}
	}
	return nil
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
