package jsonrpc2

import (
	"context"
	"testing"
)

func TestLocal(t *testing.T) {
	rpc := Local{}
	rpc.Register("", &Pinger{})

	var got string
	if err := rpc.Call(context.TODO(), &got, "ping"); err != nil {
		t.Error(err)
	}
	if want := "ping"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}
}
