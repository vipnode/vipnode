package client

import (
	"context"
	"time"

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
	nodeID, err := c.EthNode.Enode(ctx)
	if err != nil {
		return err
	}
	timestamp := time.Now()
	sig := "XXX" // TODO
	kind := ""
	nodes, err := c.Pool.Connect(ctx, sig, nodeID, timestamp, kind)
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
