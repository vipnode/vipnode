package jsonrpc2

import (
	"context"
	"testing"
)

func TestLocal(t *testing.T) {
	rpc := Local{}
	if err := rpc.Register("", &FruitService{}); err != nil {
		t.Fatal(err)
	}

	var got string
	if err := rpc.Call(context.Background(), &got, "apple"); err != nil {
		t.Error(err)
	}
	if want := "Apple"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}

	if err := rpc.Call(context.Background(), nil, "banana"); err != nil {
		t.Error(err)
	}
}
