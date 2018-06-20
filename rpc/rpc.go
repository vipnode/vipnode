package rpc

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
)

type NodeClient int

const (
	Unknown NodeClient = iota // We'll treat unknown as Geth, just in case.
	Geth
	Parity
)

func Dial(uri string) (*rpc.Client, error) {
	return rpc.Dial(uri)
}

func DetectClient(client *rpc.Client) (NodeClient, error) {
	// TODO: Detect Parity
	var info p2p.NodeInfo
	if err := client.Call(&info, "admin_nodeInfo"); err != nil {
		return Unknown, err
	}
	if strings.HasPrefix(info.Name, "Geth/") {
		return Geth, nil
	}
	return Unknown, nil
}

type gethNode struct {
	client *rpc.Client
}

func (n *gethNode) AddTrustedPeer(ctx context.Context, nodeID string) error {
	var result interface{}
	return n.client.CallContext(ctx, &result, "eth_AddTrustedPeer", nodeID)
}
