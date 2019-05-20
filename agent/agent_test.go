package agent

import (
	"context"
	"os"
	"testing"

	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/pool"
	"github.com/vipnode/vipnode/pool/store"
)

func TestAgent(t *testing.T) {
	SetLogger(os.Stderr)

	agent := Agent{
		EthNode: &fakenode.FakeNode{
			NodeID: "foo",
		},
		NumHosts: 3,
	}

	p := &pool.StaticPool{}
	if err := agent.Start(p); err != nil {
		t.Fatal(err)
	}

	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := len(peers), 0; got != want {
		t.Errorf("wrong number of peers: got %d; want %d", got, want)
	}

	p.Nodes = append(p.Nodes, store.Node{
		URI: "foo",
	})

	// Force update
	if err := agent.updatePeers(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := len(peers), 1; got != want {
		t.Errorf("wrong number of peers: got %d; want %d", got, want)
	}
}
