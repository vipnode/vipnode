package jsonrpc2

import (
	"context"
	"encoding/json"
	"net"
	"reflect"
	"testing"
)

func TestRemoteManual(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	r1 := Remote{Conn: c1}
	r1.Register("", &Pinger{})
	//go r1.Serve()

	r2 := Remote{Conn: c2}
	r2.Register("", &Ponger{})
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
	resp := r2.Server.Handle(context.TODO(), req)
	var got string
	if err := json.Unmarshal(resp.Response.Result, &got); err != nil {
		t.Error(err)
	}
	if want := "pong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}
}

func TestRemoteBidirectional(t *testing.T) {
	connPinger, connPonger := net.Pipe()
	defer connPinger.Close()
	defer connPonger.Close()

	pingerClient := Remote{Conn: connPinger}
	pongerClient := Remote{Conn: connPonger}

	ponger := &Ponger{}
	pingerClient.Register("", ponger)

	pinger := &Pinger{
		PongService: &pongerClient,
	}
	pongerClient.Register("", pinger)

	// These should serve until the connection is closed
	go pingerClient.Serve()
	go pongerClient.Serve()

	var got string
	if err := pongerClient.Call(&got, "pong"); err != nil {
		t.Error(err)
	}
	if want := "pong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}

	if got, want := pinger.PingPong(), "pingpong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}

	if err := pingerClient.Call(&got, "pingPong"); err != nil {
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

	client1 := Remote{Conn: conn2}
	client2 := Remote{Conn: conn1}

	fib := &Fib{}
	client1.Register("", fib)
	client2.Register("", fib)

	// These should serve until the connection is closed
	go client1.Serve()
	go client2.Serve()

	// 0, 1, 1, 2, 3, 5, 8, 13, 21
	var got int
	if err := client1.Call(&got, "fibonacci", 0, 1, 6); err != nil {
		t.Error(err)
	}
	if want := 21; got != want {
		t.Errorf("got: %d; want %d", got, want)
	}
}
