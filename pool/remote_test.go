package pool

import (
	"context"
	"testing"

	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool/store"
)

func TestRemotePool(t *testing.T) {
	pool := New()
	pool.skipWhitelist = true

	server, client := jsonrpc2.ServePipe()
	server.Server.Register("vipnode_", pool)

	privkey := keygen.HardcodedKey(t)
	remote := Remote(client, privkey)

	// Add self to pool first, then let's see if we're advised to connect to
	// self (this probably should error at some point but good test for now).
	if err := pool.Store.SetNode(store.Node{ID: "foo", URI: "enode://foo", IsHost: true, Kind: "geth"}, ""); err != nil {
		t.Fatal("failed to add host node:", err)
	}
	if err := pool.Store.SetNode(store.Node{ID: "bar", URI: "enode://bar", IsHost: true, Kind: "parity"}, ""); err != nil {
		t.Fatal("failed to add host node:", err)
	}

	nodes := pool.Store.GetHostNodes("", 3)
	if len(nodes) != 2 {
		t.Errorf("GetHostNodes returned unexpected number of nodes: %d", len(nodes))
	}

	hosts, err := remote.Connect(context.Background(), "geth")
	if err != nil {
		t.Error(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("wrong number of hosts: %d", len(hosts))
	}

	if hosts[0].URI != "enode://foo" {
		t.Errorf("invalid hosts result: %v", hosts)
	}
}
