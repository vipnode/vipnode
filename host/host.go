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

	if err := p.Host(context.TODO(), h.node.Kind().String(), h.uri); err != nil {
		return err
	}

	// TODO: Resume tracking peers that we care about (in case of interrupted
	// shutdown)

	ticker := time.Tick(60 * time.Second)
	for {
		select {
		case <-ticker:
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
			logger.Print("Pool.Update: %d peers, %q balance", len(peerUpdate), balance)
		case <-ctx.Done():
			return nil
		}
	}
}
