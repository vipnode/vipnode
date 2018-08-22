package jsonrpc2

import (
	"context"
	"encoding/json"
	"net"
	"reflect"
	"testing"

	"golang.org/x/sync/errgroup"
)

func TestRemoteManual(t *testing.T) {
	c1, c2 := net.Pipe()
	r1 := Remote{Codec: IOCodec(c1)}
	r2 := Remote{Codec: IOCodec(c2)}

	r1.Register("", &Pinger{})
	r2.Register("", &Ponger{})

	req, err := r1.Client.Request("pong")
	if err != nil {
		t.Error(err)
	}
	var g errgroup.Group
	g.Go(func() error {
		return r1.Codec.WriteMessage(req)
	})

	var req2 *Message
	if req2, err = r2.ReadMessage(); err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(req, req2) {
		t.Errorf("message does not match:\n  got: %s\n  want: %s", req2, req)
	}
	if err := g.Wait(); err != nil {
		t.Error(err)
	}

	resp := r2.Server.Handle(context.Background(), req)
	var got string
	if err := json.Unmarshal(resp.Response.Result, &got); err != nil {
		t.Error(err)
	}
	if want := "pong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}
}

func TestRemoteBidirectional(t *testing.T) {
	pingerClient, pongerClient := ServePipe()

	ponger := &Ponger{}
	pingerClient.Register("", ponger)

	pinger := &Pinger{
		PongService: pongerClient,
	}
	pongerClient.Register("", pinger)

	var got string
	if err := pongerClient.Call(context.Background(), &got, "pong"); err != nil {
		t.Error(err)
	}
	if want := "pong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}

	if got, want := pinger.PingPong(), "pingpong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}

	if err := pingerClient.Call(context.Background(), &got, "pingPong"); err != nil {
		t.Error(err)
	}
	if want := "pingpong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}
}

func TestRemoteContextService(t *testing.T) {
	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	client1 := Remote{Codec: IOCodec(conn2)}
	client2 := Remote{Codec: IOCodec(conn1)}

	fib := &Fib{}
	client1.Register("", fib)
	client2.Register("", fib)

	// These should serve until the connection is closed
	go client1.Serve()
	go client2.Serve()

	// 0, 1, 1, 2, 3, 5, 8, 13, 21
	var got int
	if err := client1.Call(context.Background(), &got, "fibonacci", 0, 1, 6); err != nil {
		t.Error(err)
	}
	if want := 21; got != want {
		t.Errorf("got: %d; want %d", got, want)
	}
}
