package client

import (
	"testing"

	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

func TestClient(t *testing.T) {
	client := Client{
		EthNode: &fakenode.FakeNode{
			NodeID: "foo",
		},
	}

	p := pool.StaticPool{}
	err := client.Start(&p)
	if _, ok := err.(pool.NoHostNodesError); !ok {
		t.Errorf("unexpected no nodes error, got: %q", err)
	}

	p.Nodes = append(p.Nodes, store.Node{
		URI: "foo",
	})
}
