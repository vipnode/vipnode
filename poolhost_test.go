package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool"
)

func TestPoolHost(t *testing.T) {
	privkey := keygen.HardcodedKey(t)

	p := pool.New()
	poolserver := &jsonrpc2.Local{}
	poolserver.Register("vipnode_", p)

	nodeID := discv5.PubkeyID(&privkey.PublicKey).String()
	//node := fakenode.Node(nodeID)
	nodeURI := fmt.Sprintf("enode://%s@127.0.0.1", nodeID)
	//h := host.New(nodeURI, node)
	remotePool := pool.Remote(poolserver, privkey)

	ctx := context.TODO()
	if err := remotePool.Host(ctx, "geth", "", nodeURI); err != nil {
		t.Error(err)
	}

	_, err := remotePool.Update(ctx, []string{"foo"})
	if err != nil {
		t.Error(err)
	}
}
