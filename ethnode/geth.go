package ethnode

import (
	"context"
	"strings"
)

const errCodeMethodNotFound = -32601

type codedError interface {
	error
	ErrorCode() int
}

var _ EthNode = &gethNode{}

// encodeNodeID ensures that nodeID starts with an enode:// prefix so that geth
// accepts it.
// FIXME: Should this be applied to all node implementations? Or should we be
// better about including the appropriate prefix before it's passed in?
func encodeNodeID(nodeID string) string {
	if strings.HasPrefix(nodeID, "enode://") {
		return nodeID
	}
	return "enode://" + nodeID
}

type gethNode struct {
	baseNode
}

func (n *gethNode) Kind() NodeKind {
	return Geth
}

func (n *gethNode) ConnectPeer(ctx context.Context, nodeURI string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "admin_addPeer", encodeNodeID(nodeURI))
}

func (n *gethNode) DisconnectPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "admin_removePeer", encodeNodeID(nodeID))
}

func (n *gethNode) AddTrustedPeer(ctx context.Context, nodeID string) error {
	// Result is always true, not worth checking
	var result interface{}
	return n.client.CallContext(ctx, &result, "admin_addTrustedPeer", encodeNodeID(nodeID))
}

func (n *gethNode) RemoveTrustedPeer(ctx context.Context, nodeID string) error {
	// Result is always true, not worth checking
	var result interface{}
	return n.client.CallContext(ctx, &result, "admin_removeTrustedPeer", encodeNodeID(nodeID))
}

func (n *gethNode) Peers(ctx context.Context) ([]PeerInfo, error) {
	var peers []PeerInfo
	err := n.client.CallContext(ctx, &peers, "admin_peers")
	if err != nil {
		return nil, err
	}
	return peers, nil
}

func (n *gethNode) Enode(ctx context.Context) (string, error) {
	var info struct {
		Enode string `json:"enode"` // Enode URL for adding this peer from remote peers
	}
	err := n.client.CallContext(ctx, &info, "admin_nodeInfo")
	if err != nil {
		return "", err
	}
	return info.Enode, nil
}
