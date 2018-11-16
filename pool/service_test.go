package pool

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/jsonrpc2"
	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/request"
)

func TestPoolInstance(t *testing.T) {
	pool := New(store.MemoryStore(), nil)

	r := pool.Ping(context.Background())
	if r != "pong" {
		t.Errorf("pool.Ping direct call failed: %s", r)
	}

	privkey := keygen.HardcodedKey(t)
	req := request.NodeRequest{
		Method: "vipnode_client",
		NodeID: discv5.PubkeyID(&privkey.PublicKey).String(),
		Nonce:  42,
		ExtraArgs: []interface{}{
			ClientRequest{Kind: "geth"},
		},
	}
	sig, err := req.Sign(privkey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Client(context.Background(), sig, req.NodeID, req.Nonce, req.ExtraArgs[0].(ClientRequest))
	if _, ok := err.(NoHostNodesError); !ok {
		t.Errorf("pool.Connect direct call failed: %s", err)
	}
}

func TestPoolService(t *testing.T) {
	pool := New(store.MemoryStore(), nil)
	server, client := jsonrpc2.ServePipe()
	server.Server.Register("vipnode_", pool)

	{
		var result interface{}
		if err := client.Call(context.TODO(), &result, "vipnode_ping"); err != nil {
			t.Error(err)
		}
		if result.(string) != "pong" {
			t.Errorf("invalid ping result: %s", result)
		}
	}

	privkey := keygen.HardcodedKey(t)
	req := request.NodeRequest{
		Method: "vipnode_client",
		NodeID: discv5.PubkeyID(&privkey.PublicKey).String(),
		Nonce:  42,
		ExtraArgs: []interface{}{
			ClientRequest{Kind: "geth"},
		},
	}

	{
		args, err := req.SignedArgs(privkey)
		if err != nil {
			t.Fatal(err)
		}
		var result interface{}
		if err := client.Call(context.TODO(), &result, req.Method, args...); err.Error() != (NoHostNodesError{}).Error() {
			t.Error(err)
		}
	}
}
