package fakenode

import (
	"context"
	"reflect"
	"testing"
)

func TestFakeNode(t *testing.T) {
	n := Node("foo")
	n.ConnectPeer(context.Background(), "abc")

	if len(n.Calls) != 1 {
		t.Errorf("wrong number of calls: %d", len(n.Calls))
	}

	expected := Calls{
		Call("ConnectPeer", "abc"),
	}
	if !reflect.DeepEqual(n.Calls, expected) {
		t.Errorf("got: %s; want: %s", n.Calls, expected)
	}
}
