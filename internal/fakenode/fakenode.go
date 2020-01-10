package fakenode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/vipnode/vipnode/ethnode"
)

type call struct {
	Method string
	Args   []interface{}
}

type Calls []call

func Call(method string, args ...interface{}) call {
	return call{method, args}
}

func Node(nodeID string) *FakeNode {
	return &FakeNode{
		NodeKind:   ethnode.Geth,
		NodeID:     nodeID,
		Calls:      Calls{},
		IsFullNode: true,
	}
}

// FakeNode is an implementation of ethnode.EthNode that no-ops for everything.
type FakeNode struct {
	NodeKind        ethnode.NodeKind
	NodeID          string
	Calls           Calls
	FakePeers       []ethnode.PeerInfo
	FakeBlockNumber uint64
	IsFullNode      bool
}

func (n *FakeNode) ContractBackend() bind.ContractBackend {
	return &ethclient.Client{}
}

func (n *FakeNode) NodeRPC() *rpc.Client {
	return &rpc.Client{}
}

func (n *FakeNode) UserAgent() ethnode.UserAgent {
	return ethnode.UserAgent{
		Version:    "Geth/Fakenode/Go-tests",
		Network:    1,
		IsFullNode: n.IsFullNode,
		Kind:       n.NodeKind,
	}
}
func (n *FakeNode) Kind() ethnode.NodeKind                    { return n.NodeKind }
func (n *FakeNode) Enode(ctx context.Context) (string, error) { return n.NodeID, nil }
func (n *FakeNode) AddTrustedPeer(ctx context.Context, nodeID string) error {
	n.Calls = append(n.Calls, Call("AddTrustedPeer", nodeID))
	return nil
}
func (n *FakeNode) RemoveTrustedPeer(ctx context.Context, nodeID string) error {
	n.Calls = append(n.Calls, Call("RemoveTrustedPeer", nodeID))
	return nil
}
func (n *FakeNode) ConnectPeer(ctx context.Context, nodeURI string) error {
	n.Calls = append(n.Calls, Call("ConnectPeer", nodeURI))
	uri, err := url.Parse(nodeURI)
	if err != nil {
		return err
	}
	n.FakePeers = append(n.FakePeers, ethnode.PeerInfo{
		ID:   uri.User.Username(),
		Caps: []string{"fake/1", "eth/62", "eth/63", "les/2"},
		Protocols: map[string]json.RawMessage{
			"fake": json.RawMessage("{}"),
		},
	})
	return nil
}
func (n *FakeNode) DisconnectPeer(ctx context.Context, nodeID string) error {
	n.Calls = append(n.Calls, Call("DisconnectPeer", nodeID))
	return nil
}
func (n *FakeNode) Peers(ctx context.Context) ([]ethnode.PeerInfo, error) {
	return n.FakePeers, nil
}
func (n *FakeNode) BlockNumber(ctx context.Context) (uint64, error) {
	return n.FakeBlockNumber, nil
}

func FakePeers(num int) []ethnode.PeerInfo {
	peers := make([]ethnode.PeerInfo, 0, num)
	for i := 0; i < num; i++ {
		peers = append(peers, ethnode.PeerInfo{
			ID: fmt.Sprintf("%0128x", i),
		})
	}
	return peers
}
