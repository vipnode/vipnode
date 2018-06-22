package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/rpc"
)

type parityPeers struct {
	Peers []PeerInfo `json:"peers"`
}

type parityNode struct {
	client *rpc.Client
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
	return result.Peers, nil
}
