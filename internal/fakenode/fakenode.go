package fakenode

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/vipnode/vipnode/v2/ethnode"
)

type call struct {
	Method string
	Args   []interface{}
}

type Calls []call

func Call(method string, args ...interface{}) call {
	return call{method, args}
}

// Has checks if the call list has this call
func (calls Calls) Has(method string, args ...interface{}) bool {
	want := call{method, args}
	for _, have := range calls {
		if reflect.DeepEqual(want, have) {
			return true
		}
	}
	return false
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
	uri, err := ethnode.ParseNodeURI(nodeURI)
	if err != nil {
		return err
	}
	for _, p := range n.FakePeers {
		if p.ID == uri.ID() {
			// Already present, skip
			return nil
		}
	}
	p := ethnode.PeerInfo{
		ID:   uri.ID(),
		Caps: []string{"fake/1", "eth/62", "eth/63", "les/2"},
		Protocols: map[string]json.RawMessage{
			"fake": json.RawMessage("{}"),
		},
	}
	p.Network.RemoteAddress = uri.RemoteAddress()
	n.FakePeers = append(n.FakePeers, p)
	return nil
}
func (n *FakeNode) DisconnectPeer(ctx context.Context, nodeID string) error {
	n.Calls = append(n.Calls, Call("DisconnectPeer", nodeID))
	for i, p := range n.FakePeers {
		if p.EnodeID() == nodeID {
			// Found, remove
			n.FakePeers = append(n.FakePeers[:i], n.FakePeers[i+1:]...)
		}
	}
	return nil
}
func (n *FakeNode) Peers(ctx context.Context) ([]ethnode.PeerInfo, error) {
	r := n.FakePeers[:]
	sort.Slice(r, func(i, j int) bool {
		return r[i].ID < r[j].ID
	})
	return r, nil
}
func (n *FakeNode) BlockNumber(ctx context.Context) (uint64, error) {
	return n.FakeBlockNumber, nil
}

func FakePeers(num int) []ethnode.PeerInfo {
	peers := make([]ethnode.PeerInfo, 0, num)
	for i := 0; i < num; i++ {
		p := ethnode.PeerInfo{
			ID: fmt.Sprintf("%0128x", i),
		}
		p.Network.RemoteAddress = fmt.Sprintf("192.0.2.%d:30303", i) // Remote-ish IP addresses, but still reserved.
		peers = append(peers, p)
	}
	return peers
}
