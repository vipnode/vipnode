package client

import (
	"context"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/pool"
)

// Client represents a vipnode client which connects to a vipnode host.
type Client struct {
	ethnode.EthNode
	pool.Pool
}

func (c *Client) Connect() error {
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
	return nil
}

func (c *Client) Close() error {
	return nil
}
