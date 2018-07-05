package client

import (
	"context"
	"testing"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/pool"
)

type FakeNode struct {
}

func (n *FakeNode) Kind() ethnode.NodeKind {
	return ethnode.Geth
}

func (n *FakeNode) Enode(ctx context.Context) (string, error) {
	return "foo", nil
}

func (n *FakeNode) AddTrustedPeer(ctx context.Context, nodeID string) error {
	return nil
}

func (n *FakeNode) RemoveTrustedPeer(ctx context.Context, nodeID string) error {
	return nil
}

func (n *FakeNode) ConnectPeer(ctx context.Context, nodeURI string) error {
	return nil
}

func (n *FakeNode) DisconnectPeer(ctx context.Context, nodeID string) error {
	return nil
}

func (n *FakeNode) Peers(ctx context.Context) ([]ethnode.PeerInfo, error) {
	return []ethnode.PeerInfo{}, nil
}

func TestClient(t *testing.T) {
	p := pool.StaticPool{}
	client := Client{
		EthNode: &FakeNode{},
		Pool:    &p,
	}

	if err := client.Connect(); err != pool.ErrNoHostNodes {
		t.Errorf("unexpected no nodes error, got: %q", err)
	}

	p.Nodes = append(p.Nodes, pool.HostNode{
		URI: "foo",
	})

	if err := client.Connect(); err != nil {
		t.Errorf("unexpected error: %q", err)
	}
}
