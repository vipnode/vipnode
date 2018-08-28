package main

import (
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
	payout := ""

	p := pool.New()
	rpcPool2Host, rpcHost2Pool := jsonrpc2.ServePipe()
	defer rpcPool2Host.Close()
	defer rpcHost2Pool.Close()
	if err := rpcPool2Host.Server.Register("vipnode_", p); err != nil {
		t.Fatalf("failed to register vipnode_ rpc for pool: %s", err)
	}

	hostNodeID := discv5.PubkeyID(&privkey.PublicKey).String()
	hostNode := fakenode.Node(hostNodeID)
	hostNodeURI := fmt.Sprintf("enode://%s@127.0.0.1", hostNodeID)
	h := host.New(hostNodeURI, hostNode, payout)
	if err := rpcHost2Pool.Server.RegisterMethod("vipnode_whitelist", h, "Whitelist"); err != nil {
		t.Fatalf("failed to register vipnode_ rpc for host: %s", err)
	}
	hostPool := pool.Remote(rpcHost2Pool, privkey)

	if err := h.Start(hostPool); err != nil {
		t.Fatalf("failed to start host: %s", err)
	}
	defer h.Stop()

	rpcPool2Client, rpcClient2Pool := jsonrpc2.ServePipe()
	defer rpcPool2Client.Close()
	defer rpcClient2Pool.Close()
	rpcPool2Client.Server.Register("vipnode_", p)

	clientPrivkey := keygen.HardcodedKeyIdx(t, 1)
	clientNodeID := discv5.PubkeyID(&clientPrivkey.PublicKey).String()
	clientNode := fakenode.Node(clientNodeID)
	c := client.New(clientNode)
	clientPool := pool.Remote(rpcClient2Pool, clientPrivkey)
	if err := c.Start(clientPool); err != nil {
		t.Fatalf("failed to start client: %s", err)
	}
	defer c.Stop()

	want := fakenode.Calls{
		fakenode.Call("AddTrustedPeer", clientNodeID),
	}
	if got := hostNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("hostNode.Calls:\n  got %q;\n want %q", got, want)
	}

	want = fakenode.Calls{fakenode.Call("ConnectPeer", hostNodeURI)}
	if got := clientNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("clientNode.Calls:\n  got %q;\n want %q", got, want)
	}
}
