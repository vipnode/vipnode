package agent

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/vipnode/vipnode/ethnode"
	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/pool"
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

	p.AddNode("enode://bar@127.0.0.1:30303")

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
		NumHosts:    100,
		StrictPeers: true,
	}

	fakepeers := fakenode.FakePeers(30)
	prefilled, fakepeers := fakepeers[0:10], fakepeers[10:]

	// Prefill peers
	node.FakePeers = prefilled

	// Confirm peers
	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := len(peers), 10; got != want {
		t.Errorf("wrong number of peers: got %d; want %d", got, want)
	}

	p := &pool.StaticPool{}
	// New node
	p.AddNode("enode://bar@1.1.1.1:30303")
	// Include a subset of the original
	p.AddNode(fakepeers[4].EnodeURI())
	p.AddNode(fakepeers[5].EnodeURI())
	// Mismatch host
	p.AddNode("enode://" + fakepeers[6].EnodeID() + "@42.42.42.42:30303")

	expectURIs := []string{
		fakepeers[4].EnodeURI(),
		fakepeers[5].EnodeURI(),
		"enode://" + fakepeers[6].EnodeID() + "@42.42.42.42:30303",
		"enode://bar@1.1.1.1:30303",
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

	// One more time, add more peers, change hostname of one, port of another
	if err := node.ConnectPeer(context.Background(), fakepeers[11].EnodeURI()); err != nil {
		t.Fatal(err)
	}
	if err := node.ConnectPeer(context.Background(), fakepeers[12].EnodeURI()); err != nil {
		t.Fatal(err)
	}

	p.AddNode("enode://" + fakepeers[6].EnodeID() + "@1.2.3.4:30303") // Change host
	p.AddNode("enode://bar@1.1.1.1:40404")                            // Change port

	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := ethnode.Peers(peers).URIs(), expectURIs; reflect.DeepEqual(got, want) {
		// fakepeers[6] should have the old host (as defined in expectURIs)
		t.Errorf("mismatched enode URIs:\n got: %s\nwant: %s", got, want)
	}

	// Force update
	if err := agent.UpdatePeers(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	// Update expectURIs
	expectURIs = []string{
		fakepeers[4].EnodeURI(),
		fakepeers[5].EnodeURI(),
		"enode://" + fakepeers[6].EnodeID() + "@1.2.3.4:30303",
		"enode://bar@1.1.1.1:40404",
	}

	// Set should match the pool set. This check only passes with StrictMode
	// host checking.
	if peers, err := agent.EthNode.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if got, want := ethnode.Peers(peers).URIs(), expectURIs; !reflect.DeepEqual(got, want) {
		t.Errorf("mismatched enode URIs:\n got: %s\nwant: %s", got, want)
	}
}
