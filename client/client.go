package client

import (
	"errors"

	"github.com/vipnode/vipnode/ethnode"
)

// Client represents a vipnode client which connects to a vipnode host.
type Client struct {
	ethnode.EthNode
}

func (c *Client) Connect() error {
	return errors.New("not implemented")
}

func (c *Client) Close() error {
	return errors.New("not implemented")
}
