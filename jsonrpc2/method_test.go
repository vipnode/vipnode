package jsonrpc2

import (
	"context"
	"encoding/json"
	"testing"
)

type SomeReq struct {
	Foo string `json:"foo"`
	Bar string `json:"bar"`
}

type SomeResp struct {
	Foo string `json:"foo"`
	Bar string `json:"bar"`
}

type SomeType struct{}

func (s *SomeType) Hello(prefix string, req SomeReq) (*SomeResp, error) {
	return &SomeResp{Foo: req.Foo, Bar: req.Bar}, nil
}

func TestMethodArgs(t *testing.T) {
	receiver := &SomeType{}
	m, err := MethodByName(receiver, "Hello")
	if err != nil {
		t.Fatal(err)
	}

	res, err := m.CallJSON(context.Background(), json.RawMessage(`["ignorethis", {"foo": "hello", "bar": "bye"}]`))
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := res.(*SomeResp)
	if !ok {
		t.Fatalf("invalid response type: %T", res)
	}

	if resp.Foo != "hello" || resp.Bar != "bye" {
		t.Errorf("response mismatch: %+v", resp)
	}

}
