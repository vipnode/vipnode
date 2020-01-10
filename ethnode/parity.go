package ethnode

import (
	"context"
)

var _ EthNode = &parityNode{}

type parityPeers struct {
	Peers []PeerInfo `json:"peers"`
}

type parityNode struct {
	baseNode
}

func (n *parityNode) Kind() NodeKind {
	return Parity
}

func (n *parityNode) ConnectPeer(ctx context.Context, nodeURI string) error {
	// Parity doesn't have a way to just add peers, so we overload
	// addReservedPeer for this.
	return n.AddTrustedPeer(ctx, nodeURI)
}

func (n *parityNode) DisconnectPeer(ctx context.Context, nodeID string) error {
	// Parity doesn't have a way to drop a specific peer, so we overload
	// removeReservedPeer for this.
	return n.RemoveTrustedPeer(ctx, nodeID)
}

func (n *parityNode) AddTrustedPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "parity_addReservedPeer", nodeID)
}

func (n *parityNode) RemoveTrustedPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "parity_removeReservedPeer", nodeID)
}

func (n *parityNode) Peers(ctx context.Context) ([]PeerInfo, error) {
	var result parityPeers
	err := n.client.CallContext(ctx, &result, "parity_netPeers")
	if err != nil {
		return nil, err
	}
	return filterActivePeers(result.Peers), nil
}

func (n *parityNode) Enode(ctx context.Context) (string, error) {
	var result string
	if err := n.client.CallContext(ctx, &result, "parity_enode"); err != nil {
		return "", err
	}
	return result, nil
}

// filterActivePeers filters out any peers that have not completed the
// handshake yet. In Parity, these are peers without any specified Protocols.
func filterActivePeers(peers []PeerInfo) []PeerInfo {
	if len(peers) == 0 {
		return peers
	}
	activePeers := make([]PeerInfo, 0, len(peers))
	for _, peer := range peers {
		if len(peer.Protocols) > 0 {
			activePeers = append(activePeers, peer)
		}
	}
	return activePeers
}
