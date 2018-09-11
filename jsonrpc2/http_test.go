package jsonrpc2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
)

func TestHTTPService(t *testing.T) {
	server := HTTPServer{}
	if err := server.Register("", &FruitService{}); err != nil {
		t.Fatal(err)
	}

	serverConn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer serverConn.Close()
	endpoint := fmt.Sprintf("http://%s", serverConn.Addr().String())

	errChan := make(chan error, 1)
	go func() {
		errChan <- http.Serve(serverConn, &server)
	}()

	rpc := HTTPService{
		Endpoint: endpoint,
	}

	// Try manual HTTP request first
	msg, err := rpc.Request("apple")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Error(err)
	}
	var respMsg Message
	if err := json.NewDecoder(resp.Body).Decode(&respMsg); err != nil {
		t.Fatal(err)
	}
	if got, want := string(respMsg.ID), string(msg.ID); got != want {
		t.Errorf("response ID mismatch: %q != %q", got, want)
	}
	if got, want := string(respMsg.Response.Result), `"Apple"`; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}

	// Try rpc.Call
	var got string
	if err := rpc.Call(context.Background(), &got, "apple"); err != nil {
		t.Error(err)
	}
	if want := "Apple"; got != want {
		t.Errorf("got: %q; want %q", got, want)
	}

	select {
	case err := <-errChan:
		t.Errorf("http.Serve failed: %s", err)
	default:
	}
}
