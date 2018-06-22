package rpc

import (
	"context"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
)

// NodeKind represents the different kinds of node implementations we know about.
type NodeKind int

const (
	Unknown NodeKind = iota // We'll treat unknown as Geth, just in case.
	Geth
	Parity
)

// Dial is a wrapper around go-ethereum/rpc.Dial with client detection.
func Dial(uri string) (RPC, error) {
	client, err := rpc.Dial(uri)
	if err != nil {
		return nil, err
	}

	return RemoteNode(client)
}

// DetectClient queries the RPC API to determine which kind of node is running.
func DetectClient(client *rpc.Client) (NodeKind, error) {
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

// RPC is the normalized interface between different kinds of nodes.
type RPC interface {
	// TrustPeer adds a nodeID to a set of nodes that can always connect, even
	// if the maximum number of connections is reached.
	TrustPeer(ctx context.Context, nodeID string) error
}

func RemoteNode(client *rpc.Client) (RPC, error) {
	kind, err := DetectClient(client)
	if err != nil {
		return nil, err
	}
	switch kind {
	case Parity:
		return nil, errors.New("parity not implemented yet")
	default:
		// Treat everything else as Geth
		// FIXME: Is this a bad idea?
		node := &gethNode{client: client}
		ctx := context.TODO()
		if err := node.CheckCompatible(ctx); err != nil {
			return nil, err
		}
		return node, nil
	}
}
