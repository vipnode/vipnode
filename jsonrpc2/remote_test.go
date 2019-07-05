package jsonrpc2

import (
	"context"
	"encoding/json"
	"net"
	"reflect"
	"sort"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func TestRemoteManual(t *testing.T) {
	c1, c2 := net.Pipe()
	s1, s2 := Server{}, Server{}
	r1 := Remote{Codec: IOCodec(c1), Server: &s1, Client: &Client{}}
	r2 := Remote{Codec: IOCodec(c2), Server: &s2, Client: &Client{}}

	s1.Register("", &Pinger{})
	s2.Register("", &Ponger{})

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

	assertEqualJSON(t, req2, req, "message does not match")
	if err := g.Wait(); err != nil {
		t.Error(err)
	}

	resp := r2.Server.Handle(context.Background(), req.Request)
	var got string
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Error(err)
	}
	if want := "pong"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}
}

func TestRemoteBidirectional(t *testing.T) {
	pingerClient, pongerClient := ServePipe()

	ponger := &Ponger{}
	pingerClient.Server.Register("", ponger)

	pinger := &Pinger{
		PongService: pongerClient,
	}
	pongerClient.Server.Register("", pinger)

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

	s1, s2 := Server{}, Server{}
	client1 := Remote{Codec: IOCodec(conn1), Server: &s1, Client: &Client{}}
	client2 := Remote{Codec: IOCodec(conn2), Server: &s2, Client: &Client{}}

	fib := &Fib{}
	s1.Register("", fib)
	s2.Register("", fib)

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

func TestRemoteCleanPending(t *testing.T) {
	r := Remote{
		PendingLimit:   5,
		PendingDiscard: 3,
	}
	now := time.Now().Add(-time.Second * 100)
	r.pending = map[string]pendingMsg{
		"1": pendingMsg{timestamp: now.Add(time.Second * 1)},
		"2": pendingMsg{timestamp: now.Add(time.Second * 2)},
		"3": pendingMsg{timestamp: now.Add(time.Second * 3)},
		"4": pendingMsg{timestamp: now.Add(time.Second * 4)},
		"5": pendingMsg{timestamp: now.Add(time.Second * 5)},
	}

	if want, got := 5, len(r.pending); got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}

	// Should trigger a cleanup of 3, add 1.
	r.getPendingChan("6")
	if want, got := 3, len(r.pending); got != want {
		t.Errorf("got: %d; want: %d", got, want)
	}

	keys := []string{}
	for k, _ := range r.pending {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if want, got := []string{"4", "5", "6"}, keys; !reflect.DeepEqual(got, want) {
		t.Errorf("got: %q; want %q", got, want)
	}
}

func TestRemoteServeInvalid(t *testing.T) {
	pingerClient, pongerClient := ServePipe()

	ponger := &Ponger{}
	pingerClient.Server.Register("", ponger)

	emptyMsg := &Message{}
	if err := pongerClient.WriteMessage(emptyMsg); err != nil {
		t.Error(err)
	}
}
