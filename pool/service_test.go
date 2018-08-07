package pool

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/vipnode/vipnode/internal/keygen"
	"github.com/vipnode/vipnode/request"
)

func TestPoolInstance(t *testing.T) {
	pool := New()

	r := pool.Ping(context.Background())
	if r != "pong" {
		t.Errorf("pool.Ping direct call failed: %s", r)
	}

	privkey := keygen.HardcodedKey(t)
	req := request.Request{
		Method: "vipnode_connect",
		NodeID: discv5.PubkeyID(&privkey.PublicKey).String(),
		Nonce:  42,
		ExtraArgs: []interface{}{
			"geth",
		},
	}
	sig, err := req.Sign(privkey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Connect(context.Background(), sig, req.NodeID, req.Nonce, req.ExtraArgs[0].(string))
	if err != ErrNoHostNodes {
		t.Errorf("pool.Connect direct call failed: %s", err)
	}
}

func TestPoolService(t *testing.T) {
	pool := New()
	server := rpc.NewServer()
	if err := server.RegisterName("vipnode", pool); err != nil {
		t.Fatal(err)
	}

	client := rpc.DialInProc(server)
	{
		var result interface{}
		if err := client.Call(&result, "vipnode_ping"); err != nil {
			t.Error(err)
		}
		if result.(string) != "pong" {
			t.Errorf("invalid ping result: %s", result)
		}
	}

	privkey := keygen.HardcodedKey(t)
	req := request.Request{
		Method: "vipnode_connect",
		NodeID: discv5.PubkeyID(&privkey.PublicKey).String(),
		Nonce:  42,
		ExtraArgs: []interface{}{
			"geth",
		},
	}

	{
		args, err := req.SignedArgs(privkey)
		if err != nil {
			t.Fatal(err)
		}
		var result interface{}
		if err := client.Call(&result, req.Method, args...); err.Error() != ErrNoHostNodes.Error() {
			t.Error(err)
		}
	}
}
