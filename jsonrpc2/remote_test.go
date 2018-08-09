package jsonrpc2

import (
	"encoding/json"
	"net"
	"reflect"
	"testing"
)

type Foo struct{}

func (f *Foo) Ping() string {
	return "ping"
}

type Bar struct{}

func (b *Bar) Pong() string {
	return "pong"
}

func TestRemoteManual(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	r1 := Remote{Conn: c1}
	r1.Register("", &Foo{})
	//go r1.Serve()

	r2 := Remote{Conn: c2}
	r2.Register("", &Bar{})
	//go r2.Serve()

	req, err := r1.Client.Request("pong")
	if err != nil {
		t.Error(err)
	}

	go json.NewEncoder(c1).Encode(req)
	var req2 Message
	if err := json.NewDecoder(c2).Decode(&req2); err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(req, &req2) {
		t.Errorf("message does not match:\n  got: %s\n  want: %s", req2, req)
	}
	resp := r2.Server.Handle(req)
	var got string
	if err := json.Unmarshal(resp.Response.Result, &got); err != nil {
		t.Error(err)
	}
	if want := "pong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}
}

func TestRemoteServing(t *testing.T) {
	connFoo, connBar := net.Pipe()
	defer connFoo.Close()
	defer connBar.Close()

	remoteFoo := Remote{Conn: connFoo}
	remoteFoo.Register("", &Bar{})
	go func() {
		if err := remoteFoo.Serve(); err != nil {
			t.Fatal(err)
		}
	}()

	remoteBar := Remote{Conn: connBar}
	remoteBar.Register("", &Foo{})
	go func() {
		if err := remoteBar.Serve(); err != nil {
			t.Fatal(err)
		}
	}()

	var got string
	if err := remoteFoo.Call(&got, "ping"); err != nil {
		t.Error(err)
	}
	if want := "ping"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}

	// FIXME: This fails right now (timeout)
	if err := remoteBar.Call(&got, "pong"); err != nil {
		t.Error(err)
	}
	if want := "pong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}
}
