package host

import (
	"context"
	"time"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

var startTimeout = 10 * time.Second
var updateTimeout = 10 * time.Second

type nodeID string

type client struct {
	nodeID nodeID
	expire time.Time
}

func New(node ethnode.EthNode, payout string) *Host {
	return &Host{
		node:   node,
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
	// NodeURI can be used to set the enode:// connection string that
	// the pool should advertise to clients. Normally, the pool will
	// automatically deduce this string from the connection IP and nodeID, but
	// we can provide an override if there is a non-standard port or if the
	// node runs on a different IP from the vipnode agent.
	NodeURI string

	node   ethnode.EthNode
	payout string
	stopCh chan struct{}
	waitCh chan error
}

// Whitelist a client for this host.
func (h *Host) Whitelist(ctx context.Context, nodeID string) error {
	logger.Printf("Received whitelist request: %s", nodeID)
	return h.node.AddTrustedPeer(ctx, nodeID)
}

// Disconnect a client from this host and remove from whitelist.
func (h *Host) Disconnect(ctx context.Context, nodeID string) error {
	logger.Printf("Received disconnect request: %s", nodeID)
	if err := h.node.RemoveTrustedPeer(ctx, nodeID); err != nil {
		return err
	}
	return h.node.DisconnectPeer(ctx, nodeID)
}

func (h *Host) updatePeers(ctx context.Context, p pool.Pool) error {
	block, err := h.node.BlockNumber(ctx)
	if err != nil {
		return err
	}

	peers, err := h.node.Peers(ctx)
	if err != nil {
		return err
	}
	peerUpdate := make([]string, 0, len(peers))
	for _, peer := range peers {
		peerUpdate = append(peerUpdate, peer.ID)
	}
	update, err := p.Update(ctx, pool.UpdateRequest{
		Peers:       peerUpdate,
		BlockNumber: block,
	})
	if err != nil {
		return err
	}
	if len(update.InvalidPeers) == 0 {
		logger.Printf("Sent update: %d peers. Pool response: %s", len(peerUpdate), update.Balance.String())
		return nil
	}
	logger.Printf("Sent update: %d peers. Pool response: Disconnect from %d invalid peers, %s", len(peerUpdate), len(update.InvalidPeers), update.Balance.String())
	for _, peerID := range update.InvalidPeers {
		// FIXME: Are there recoverable errors here?
		if err := h.node.RemoveTrustedPeer(ctx, peerID); err != nil {
			return err
		}
		if err := h.node.DisconnectPeer(ctx, peerID); err != nil {
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
	startCtx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()

	enode, err := h.node.Enode(startCtx)
	if err != nil {
		return err
	}
	logger.Printf("Connected to local node: %s", enode)

	hostReq := pool.HostRequest{
		Kind:    h.node.Kind().String(),
		Payout:  h.payout,
		NodeURI: h.NodeURI,
	}
	resp, err := p.Host(startCtx, hostReq)
	if err != nil {
		return err
	}
	logger.Printf("Registered on pool: Version %s", resp.PoolVersion)

	// TODO: Resume tracking peers that we care about (in case of interrupted
	// shutdown)?

	if err := h.updatePeers(startCtx, p); err != nil {
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
			ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
			err := h.updatePeers(ctx, p)
			cancel()
			if err != nil {
				return err
			}
		case <-h.stopCh:
			return nil
		}
	}
}
