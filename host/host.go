package host

import (
	"context"
	"time"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/pool"
)

type nodeID string

type client struct {
	nodeID nodeID
	expire time.Time
}

func New(nodeURI string, node ethnode.EthNode) *Host {
	return &Host{
		node: node,
		uri:  nodeURI,
	}
}

// Host represents a single vipnode host.
type Host struct {
	node ethnode.EthNode
	uri  string
}

// Whitelist a client for this host.
func (h *Host) Whitelist(ctx context.Context, nodeID string) error {
	return h.node.AddTrustedPeer(ctx, nodeID)
}

func (h *Host) ServeUpdates(ctx context.Context, p pool.Pool) error {
	enode, err := h.node.Enode(context.TODO())
	if err != nil {
		return err
	}
	logger.Print("Connected to node: ", enode)

	// TODO: Send wallet address too
	if err := p.Host(ctx, h.node.Kind().String(), h.uri); err != nil {
		return err
	}
	logger.Print("Registered on pool.")

	// TODO: Resume tracking peers that we care about (in case of interrupted
	// shutdown)

	updatePeers := func() error {
		peers, err := h.node.Peers(ctx)
		if err != nil {
			return err
		}
		peerUpdate := make([]string, 0, len(peers))
		for _, peer := range peers {
			peerUpdate = append(peerUpdate, peer.ID)
		}
		balance, err := p.Update(ctx, peerUpdate)
		if err != nil {
			return err
		}
		logger.Print("Sent pool update: %d peers; received balance: %q", len(peerUpdate), balance)
		return nil
	}

	if err := updatePeers(); err != nil {
		return err
	}

	ticker := time.Tick(60 * time.Second)
	for {
		select {
		case <-ticker:
			if err := updatePeers(); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}
