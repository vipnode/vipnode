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
	rpcPool2Host, rpcHost2Pool := jsonrpc2.ServePipe()
	rpcPool2Host.Register("vipnode_", p)

	hostNodeID := discv5.PubkeyID(&privkey.PublicKey).String()
	hostNode := fakenode.Node(hostNodeID)
	hostNodeURI := fmt.Sprintf("enode://%s@127.0.0.1", hostNodeID)
	h := host.New(hostNodeURI, hostNode, "")
	rpcHost2Pool.Register("vipnode_", h)
	remotePool := pool.Remote(rpcHost2Pool, privkey)

	ctx := context.Background()
	if err := remotePool.Host(ctx, kind, payout, hostNodeURI); err != nil {
		t.Error(err)
	}

	_, err := remotePool.Update(ctx, []string{"foo"})
	if err != nil {
		t.Error(err)
	}

	rpcPool2Client, rpcClient2Pool := jsonrpc2.ServePipe()
	rpcPool2Client.Register("vipnode_", p)

	clientPrivkey := keygen.HardcodedKeyIdx(t, 1)
	clientNodeID := discv5.PubkeyID(&clientPrivkey.PublicKey).String()
	clientNode := fakenode.Node(clientNodeID)
	client := client.Client{
		EthNode: clientNode,
		Pool:    pool.Remote(rpcClient2Pool, clientPrivkey),
	}

	if err := client.Connect(); err != nil {
		t.Error(err)
	}

	want := fakenode.Calls{
		fakenode.Call("AddTrustedPeer", clientNodeID),
	}
	if got := hostNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("node.Calls:\n  got %q;\n want %q", got, want)
	}

	want = fakenode.Calls{fakenode.Call("ConnectPeer", hostNodeURI)}
	if got := clientNode.Calls; !reflect.DeepEqual(got, want) {
		t.Errorf("node.Calls:\n  got %q;\n want %q", got, want)
	}
}
