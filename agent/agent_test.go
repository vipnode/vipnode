package agent

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/vipnode/vipnode/ethnode"
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
	defer agent.Stop()

	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := len(peers), 0; got != want {
		t.Errorf("wrong number of peers: got %d; want %d", got, want)
	}

	p.Nodes = append(p.Nodes, store.Node{
		URI: "enode://bar@127.0.0.1:30303",
	})

	// Force update
	if err := agent.UpdatePeers(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := len(peers), 1; got != want {
		t.Errorf("wrong number of peers: got %d; want %d", got, want)
	}
}

func TestAgentStrictPeers(t *testing.T) {
	SetLogger(os.Stderr)

	node := fakenode.Node("foo")
	agent := Agent{
		EthNode:     node,
		NumHosts:    5,
		StrictPeers: true,
	}

	fakepeers := fakenode.FakePeers(15)

	// Prefill peers
	node.FakePeers = fakepeers[:10]

	// Confirm peers
	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := len(peers), 10; got != want {
		t.Errorf("wrong number of peers: got %d; want %d", got, want)
	}

	p := &pool.StaticPool{}
	p.Nodes = append(
		p.Nodes,
		// New node
		store.Node{URI: "enode://bar@127.0.0.1:30303"},
		// Include a subset of the original
		store.Node{URI: fakepeers[4].EnodeURI()},
		store.Node{URI: fakepeers[5].EnodeURI()},
		// Mismatch host
		store.Node{URI: "enode://" + fakepeers[6].EnodeID() + "@42.42.42.42:30303"},
	)

	expectURIs := make([]string, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		expectURIs = append(expectURIs, n.URI)
	}

	if err := agent.Start(p); err != nil {
		t.Fatal(err)
	}
	defer agent.Stop()

	// Force update
	if err := agent.UpdatePeers(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	// Set should match the pool set
	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := len(peers), len(expectURIs); got != want {
		t.Errorf("wrong number of peers: got %d; want %d", got, want)
	} else if got, want := ethnode.Peers(peers).URIs(), expectURIs; !reflect.DeepEqual(got, want) {
		t.Errorf("mismatched enode URIs:\n got: %s\nwant: %s", got, want)
	}

	// One more time, add more peers
	if err := node.ConnectPeer(context.Background(), fakepeers[11].EnodeURI()); err != nil {
		t.Fatal(err)
	}
	if err := node.ConnectPeer(context.Background(), fakepeers[12].EnodeURI()); err != nil {
		t.Fatal(err)
	}
	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := len(peers), len(expectURIs)+2; got != want {
		t.Errorf("wrong number of peers: got %d; want %d", got, want)
	}

	// Force update
	if err := agent.UpdatePeers(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	// Set should match the pool set
	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := len(peers), len(expectURIs); got != want {
		t.Errorf("wrong number of peers: got %d; want %d", got, want)
	} else if got, want := ethnode.Peers(peers).URIs(), expectURIs; !reflect.DeepEqual(got, want) {
		t.Errorf("mismatched enode URIs:\n got: %s\nwant: %s", got, want)
	}
}
