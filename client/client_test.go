package client

import (
	"os"
	"testing"

	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

func TestClient(t *testing.T) {
	SetLogger(os.Stderr)

	client := New(&fakenode.FakeNode{
		NodeID: "foo",
	})

	p := pool.StaticPool{}
	err := client.Start(&p)
	if _, ok := err.(pool.NoHostNodesError); !ok {
		t.Errorf("expected no nodes error, got: %q", err)
	}

	p.Nodes = append(p.Nodes, store.Node{
		URI: "foo",
	})
}
