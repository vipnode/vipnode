package host

import (
	"errors"
	"time"
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
	// TODO: ...
	return nil
}
