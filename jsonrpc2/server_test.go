package jsonrpc2

import (
	"context"
	"encoding/json"
	"testing"
)

func TestServer(t *testing.T) {
	service := &FruitService{}
	s := Server{}
	if err := s.Register("foo_", service); err != nil {
		t.Error(err)
	}

	resp := s.Handle(context.TODO(), &Message{
		ID:      json.RawMessage([]byte("1")),
		Version: Version,
		Request: &Request{
			Method: "foo_apple",
		},
	})
	if resp.Error != nil {
		t.Errorf("unexpected error: %q", resp)
	}

	if string(resp.Result) != `"Apple"` {
		t.Errorf("unexpected result: %q", resp.Result)
	}

	resp = s.Handle(context.TODO(), &Message{
		ID:      json.RawMessage([]byte("2")),
		Version: Version,
		Request: &Request{
			Method: "foo_banana",
		},
	})
	if resp.Error != nil {
		t.Errorf("unexpected error: %q", resp)
	}

	if string(resp.Response.Result) != "null" {
		t.Errorf("unexpected result: %q", resp.Result)
	}
}
