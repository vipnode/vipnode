package fakenode

import (
	"context"
	"reflect"
	"testing"
)

func TestFakeNode(t *testing.T) {
	n := Node("foo")
	if err := n.ConnectPeer(context.Background(), "enode://abc@127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	if len(n.Calls) != 1 {
		t.Errorf("wrong number of calls: %d", len(n.Calls))
	}

	expected := Calls{
		Call("ConnectPeer", "enode://abc@127.0.0.1"),
	}
	if !reflect.DeepEqual(n.Calls, expected) {
		t.Errorf("got: %s; want: %s", n.Calls, expected)
	}

	if peers, err := n.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if len(peers) != 1 {
		t.Errorf("wrong number of peers: %s", peers)
	}

	if err := n.DisconnectPeer(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}

	if peers, err := n.Peers(context.Background()); err != nil {
		t.Fatal(err)
	} else if len(peers) != 0 {
		t.Errorf("wrong number of peers: %s", peers)
	}
}
