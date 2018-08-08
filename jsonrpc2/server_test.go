package jsonrpc2

import (
	"encoding/json"
	"errors"
	"testing"
)

type SomeService struct{}

func (_ *SomeService) Apple() string {
	return "Apple"
}

func (_ *SomeService) Banana() error {
	return nil
}

func (_ *SomeService) Cherry() (string, error) {
	return "Cherry", nil
}

func (_ *SomeService) Durian() error {
	return errors.New("durian failure")
}

func TestServer(t *testing.T) {
	service := &SomeService{}
	s := Server{}
	if err := s.Register("foo_", service); err != nil {
		t.Error(err)
	}

	resp := s.Handle(&Request{
		ID:      json.RawMessage([]byte("1")),
		Version: Version,
		Method:  "foo_apple",
	})
	if resp.Error != nil {
		t.Errorf("unexpected error: %q", resp)
	}

	if string(resp.Result) != `"Apple"` {
		t.Errorf("unexpected result: %q", resp.Result)
	}

	resp = s.Handle(&Request{
		ID:      json.RawMessage([]byte("2")),
		Version: Version,
		Method:  "foo_banana",
	})
	if resp.Error != nil {
		t.Errorf("unexpected error: %q", resp)
	}

	if resp.Result != nil {
		t.Errorf("unexpected result: %q", resp.Result)
	}
}
