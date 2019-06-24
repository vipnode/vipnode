package ethnode

import "context"

var _ EthNode = &pantheonNode{}

type pantheonNode struct {
	baseNode
}

func (n *pantheonNode) Kind() NodeKind {
	return Pantheon
}

func (n *pantheonNode) ConnectPeer(ctx context.Context, nodeURI string) error {
	// Pantheon doesn't have a way to just add peers, so we overload
	// addReservedPeer for this.
	return n.AddTrustedPeer(ctx, nodeURI)
}

func (n *pantheonNode) DisconnectPeer(ctx context.Context, nodeID string) error {
	// Pantheon doesn't have a way to drop a specific peer, so we overload
	// removeReservedPeer for this.
	return n.RemoveTrustedPeer(ctx, nodeID)
}

func (n *pantheonNode) AddTrustedPeer(ctx context.Context, nodeID string) error {
	// Result is true if the node is not already added, but we don't care.
	var result bool
	return n.client.CallContext(ctx, &result, "admin_addPeer", nodeID)
}

func (n *pantheonNode) RemoveTrustedPeer(ctx context.Context, nodeID string) error {
	// Result is true if the node was present, but we don't care.
	var result bool
	return n.client.CallContext(ctx, &result, "admin_removePeer", nodeID)
}

func (n *pantheonNode) Peers(ctx context.Context) ([]PeerInfo, error) {
	var peers []PeerInfo
	err := n.client.CallContext(ctx, &peers, "admin_peers")
	if err != nil {
		return nil, err
	}
	return peers, nil
}

func (n *pantheonNode) Enode(ctx context.Context) (string, error) {
	var enode string
	// Non-standard Pantheon-only net_* RPC? https://docs.pantheon.pegasys.tech/en/latest/Reference/Pantheon-API-Methods/#net_enode
	err := n.client.CallContext(ctx, &enode, "net_enode")
	return enode, err
}
