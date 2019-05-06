package ethnode

import (
	"context"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

var _ EthNode = &parityNode{}

type parityPeers struct {
	Peers []PeerInfo `json:"peers"`
}

type parityNode struct {
	client *rpc.Client
}

func (n *parityNode) ContractBackend() bind.ContractBackend {
	return ethclient.NewClient(n.client)
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
	// FIXME: Only return connected peers who completed the handshake? In that
	// case, need to filter by non-empty Protocols
	return result.Peers, nil
}

func (n *parityNode) Enode(ctx context.Context) (string, error) {
	var result string
	if err := n.client.CallContext(ctx, &result, "parity_enode"); err != nil {
		return "", err
	}
	return result, nil
}

func (n *parityNode) BlockNumber(ctx context.Context) (uint64, error) {
	var result string
	if err := n.client.CallContext(ctx, &result, "eth_blockNumber"); err != nil {
		return 0, err
	}
	return strconv.ParseUint(result, 0, 64)
}
