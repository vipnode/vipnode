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
		node:   node,
		uri:    nodeURI,
		payout: payout,
		stopCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}
}

type HostService interface {
	Whitelist(ctx context.Context, nodeID string) error
}

// Host represents a single vipnode host.
type Host struct {
	node   ethnode.EthNode
	uri    string
	payout string
	stopCh chan struct{}
	waitCh chan error
}

// Whitelist a client for this host.
func (h *Host) Whitelist(ctx context.Context, nodeID string) error {
	logger.Printf("Received whitelist request: %s", nodeID)
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

// Wait blocks until the host is stopped. It returns any errors that occur
// during stopping.
func (h *Host) Wait() error {
	return <-h.waitCh
}

// Start registers the host on the given pool and starts sending peer updates
// every store.KeepaliveInterval. It returns after
// successfully registering with the pool.
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

	go func() {
		h.waitCh <- h.serveUpdates(p)
	}()
	return nil
}

func (h *Host) serveUpdates(p pool.Pool) error {
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
