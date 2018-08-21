package client

import (
	"testing"

	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

func TestClient(t *testing.T) {
	p := pool.StaticPool{}
	client := Client{
		EthNode: &fakenode.FakeNode{
			NodeID: "foo",
		},
		Pool: &p,
	}

	err := client.Connect()
	if _, ok := err.(pool.ErrNoHostNodes); !ok {
		t.Errorf("unexpected no nodes error, got: %q", err)
	}

	p.Nodes = append(p.Nodes, store.HostNode{
		URI: "foo",
	})

	if err := client.Connect(); err != nil {
		t.Errorf("unexpected error: %q", err)
	}
}
