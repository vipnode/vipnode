package host

import (
	"context"
	"errors"
	"time"

	"github.com/vipnode/vipnode/ethnode"
)

type nodeID string

type client struct {
	nodeID nodeID
	expire time.Time
}

func New(node HostNode) *Host {
	return &Host{
		node:      node,
		whitelist: make(map[nodeID]time.Time),
	}
}

// HostNode represents the normalized interface required by the Ethereum node
// to support a vipnode host.
type HostNode interface {
	// FIXME: Should we just use ethnode.RPC or should each host/client package
	// specify a subset interface?
	ethnode.EthNode
}

// Host represents a single vipnode host.
type Host struct {
	node HostNode

	whitelist map[nodeID]time.Time
}

// Whitelist a client for this host.
func (h *Host) Whitelist(nodeID nodeID, expire time.Time) error {
	//e.node.Whitelist(nodeID)
	return errors.New("not implemented")
}

func (h *Host) Start() error {
	peers, err := h.node.Peers(context.TODO())
	if err != nil {
		return err
	}
	logger.Print("Received peers: ", len(peers))
	// TODO: Resume tracking peers that we care about.
	return nil
}
