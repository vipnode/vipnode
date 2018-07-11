package ethnode

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/rpc"
)

// NodeKind represents the different kinds of node implementations we know about.
type NodeKind int

const (
	Unknown NodeKind = iota // We'll treat unknown as Geth, just in case.
	Geth
	Parity
)

func (n NodeKind) String() string {
	switch n {
	case Geth:
		return "geth"
	case Parity:
		return "parity"
	}
	return "unknown"
}

// Dial is a wrapper around go-ethereum/rpc.Dial with client detection.
func Dial(uri string) (EthNode, error) {
	client, err := rpc.Dial(uri)
	if err != nil {
		return nil, err
	}

	return RemoteNode(client)
}

// DetectClient queries the RPC API to determine which kind of node is running.
func DetectClient(client *rpc.Client) (NodeKind, error) {
	// TODO: Detect Parity
	var clientVersion string
	if err := client.Call(&clientVersion, "web3_clientVersion"); err != nil {
		return Unknown, err
	}
	if strings.HasPrefix(clientVersion, "Parity/") {
		return Parity, nil
	} else if strings.HasPrefix(clientVersion, "Geth/") {
		return Geth, nil
	}
	return Unknown, nil
}

// PeerInfo stores the node ID and client metadata about a peer.
type PeerInfo struct {
	ID   string `json:"id"`   // Unique node identifier (also the encryption pubkey)
	Name string `json:"name"` // Name of the node, including client type, version, OS, custom data
}

// EthNode is the normalized interface between different kinds of nodes.
type EthNode interface {
	// Kind returns the kind of node this is.
	Kind() NodeKind
	// Enode returns this node's enode://...
	Enode(ctx context.Context) (string, error)
	// AddTrustedPeer adds a nodeID to a set of nodes that can always connect, even
	// if the maximum number of connections is reached.
	AddTrustedPeer(ctx context.Context, nodeID string) error
	// RemoveTrustedPeer removes a nodeID from the trusted node set.
	RemoveTrustedPeer(ctx context.Context, nodeID string) error
	// ConnectPeer prompts a connection to the given nodeURI.
	ConnectPeer(ctx context.Context, nodeURI string) error
	// DisconnectPeer disconnects from the given nodeID, if connected.
	DisconnectPeer(ctx context.Context, nodeID string) error
	// Peers returns the list of connected peers
	Peers(ctx context.Context) ([]PeerInfo, error)
}

// RemoteNode autodetects the node kind and returns the appropriate EthNode
// implementation.
func RemoteNode(client *rpc.Client) (EthNode, error) {
	kind, err := DetectClient(client)
	if err != nil {
		return nil, err
	}
	switch kind {
	case Parity:
		return &parityNode{client: client}, nil
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
