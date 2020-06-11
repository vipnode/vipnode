package gobwas

import (
	"net"
	"testing"

	"github.com/vipnode/vipnode/v2/jsonrpc2"
)

func TestWebSocketCodec(t *testing.T) {
	c1, c2 := net.Pipe()

	clientCodec := clientWebSocketCodec(c1)
	serverCodec := serverWebSocketCodec(c2)

	go clientCodec.WriteMessage(&jsonrpc2.Message{Version: "foo"})
	msg, err := serverCodec.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Version != "foo" {
		t.Errorf("wrong message: %v", msg)
	}

	go serverCodec.WriteMessage(&jsonrpc2.Message{Version: "bar"})
	msg, err = clientCodec.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Version != "bar" {
		t.Errorf("wrong message: %v", msg)
	}
}
