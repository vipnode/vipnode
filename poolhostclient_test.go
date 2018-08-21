package main

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/client"
	"github.com/vipnode/vipnode/host"
	"github.com/vipnode/vipnode/internal/fakenode"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool"
)

func TestPoolHostClient(t *testing.T) {
	privkey := keygen.HardcodedKeyIdx(t, 0)
	kind := "geth"
	payout := ""

	p := pool.New()
	poolserver := &jsonrpc2.Local{}
	poolserver.Register("vipnode_", p)

	nodeID := discv5.PubkeyID(&privkey.PublicKey).String()
	node := fakenode.Node(nodeID)
	nodeURI := fmt.Sprintf("enode://%s@127.0.0.1", nodeID)
	h := host.New(nodeURI, node, "")
	remotePool := pool.Remote(poolserver, privkey)

	poolserver.Register("vipnode_", h) // Eeeh let's use the same service for the Host.

	ctx := context.Background()
	if err := remotePool.Host(ctx, kind, payout, nodeURI); err != nil {
		t.Error(err)
	}

	_, err := remotePool.Update(ctx, []string{"foo"})
	if err != nil {
		t.Error(err)
	}

	// Try to fake connecting through the RPC

	hosts, err := remotePool.Connect(context.Background(), "geth")
	if err != nil {
		t.Error(err)
	}

	want := fakenode.Calls{fakenode.Call("AddTrustedPeer", nodeID)}
	if !reflect.DeepEqual(node.Calls, want) {
		t.Errorf("node.Calls:\n  got %q;\n want %q", node.Calls, want)
	}

	if len(hosts) != 1 {
		t.Fatalf("wrong number of hosts: %d", len(hosts))
	}

	if hosts[0].URI != nodeURI {
		t.Errorf("wrong host returned: %s", hosts[0].URI)
	}

	// Actual connecting with a client wrapper

	clientPrivkey := keygen.HardcodedKeyIdx(t, 1)
	clientNodeID := discv5.PubkeyID(&clientPrivkey.PublicKey).String()
	clientNode := fakenode.Node(clientNodeID)
	client := client.Client{
		EthNode: clientNode,
		Pool:    pool.Remote(poolserver, clientPrivkey),
	}

	if err := client.Connect(); err != nil {
		t.Error(err)
	}

	want = fakenode.Calls{
		fakenode.Call("AddTrustedPeer", nodeID),
		fakenode.Call("AddTrustedPeer", clientNodeID),
	}
	if got := node.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("node.Calls:\n  got %q;\n want %q", got, want)
	}

	want = fakenode.Calls{fakenode.Call("ConnectPeer", nodeURI)}
	if got := clientNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("node.Calls:\n  got %q;\n want %q", got, want)
	}
}
