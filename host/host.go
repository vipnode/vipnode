package host

import (
	"context"
	"time"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

type nodeID string

type client struct {
	nodeID nodeID
	expire time.Time
}

func New(nodeURI string, node ethnode.EthNode, payout string) *Host {
	return &Host{
		node:    node,
		uri:     nodeURI,
		payout:  payout,
		stopCh:  make(chan struct{}),
		readyCh: make(chan struct{}, 1),
	}
}

// Host represents a single vipnode host.
type Host struct {
	node    ethnode.EthNode
	uri     string
	payout  string
	stopCh  chan struct{}
	readyCh chan struct{}
}

// Whitelist a client for this host.
func (h *Host) Whitelist(ctx context.Context, nodeID string) error {
	logger.Print("Received whitelist request: %s", nodeID)
	return h.node.AddTrustedPeer(ctx, nodeID)
}

func (h *Host) updatePeers(ctx context.Context, p pool.Pool) error {
	peers, err := h.node.Peers(ctx)
	if err != nil {
		return err
	}
	peerUpdate := make([]string, 0, len(peers))
	for _, peer := range peers {
		peerUpdate = append(peerUpdate, peer.ID)
	}
	update, err := p.Update(ctx, peerUpdate)
	if err != nil {
		return err
	}
	if len(update.InvalidPeers) == 0 {
		logger.Printf("Sent pool update: %d peers; Current balance: %s", len(peerUpdate), update.Balance)
		return nil
	}
	logger.Printf("Sent pool update: %d peers; Disconnecting from %d invalid peers. Current balance: %s", len(peerUpdate), len(update.InvalidPeers), update.Balance)
	for _, peerID := range update.InvalidPeers {
		if err := h.node.RemoveTrustedPeer(ctx, peerID); err != nil {
			// FIXME: Are there recoverable errors here?
			return err
		}
	}
	return nil
}

// Stop will terminate the update peers loop, which will cause Start to return.
func (h *Host) Stop() {
	h.stopCh <- struct{}{}
}

// Ready returns a channel that yields when the host is registered on the pool,
// after the first peers update is sent.
func (h *Host) Ready() <-chan struct{} {
	return h.readyCh
}

// Start registers the host on the given pool and starts sending peer updates
// every store.KeepaliveInterval.
func (h *Host) Start(p pool.Pool) error {
	startCtx := context.Background()
	enode, err := h.node.Enode(startCtx)
	if err != nil {
		return err
	}
	logger.Printf("Connected to local node: %s", enode)

	// TODO: Send wallet address too
	if err := p.Host(startCtx, h.node.Kind().String(), h.payout, h.uri); err != nil {
		return err
	}
	logger.Print("Registered on pool.")

	// TODO: Resume tracking peers that we care about (in case of interrupted
	// shutdown)

	if err := h.updatePeers(context.Background(), p); err != nil {
		return err
	}

	h.readyCh <- struct{}{}

	ticker := time.Tick(store.KeepaliveInterval)
	for {
		select {
		case <-ticker:
			if err := h.updatePeers(context.Background(), p); err != nil {
				return err
			}
		case <-h.stopCh:
			return nil
		}
	}
}
