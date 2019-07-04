package jsonrpc2

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"sort"
	"testing"
)

type nopCloser struct {
	io.ReadWriter
}

func (nopCloser) Close() error { return nil }

func TestCodec(t *testing.T) {
	var buf bytes.Buffer
	codec := IOCodec(nopCloser{&buf})
	msg := &Message{
		ID:      []byte("42"),
		Version: "2.0",
	}
	err := codec.WriteMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	msg2, err := codec.ReadMessage()
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(msg, msg2) {
		t.Errorf("got: %q; want %q", msg2, msg)
	}
}

func TestBatch(t *testing.T) {
	tests := []struct {
		In   string
		Want []Message
	}{
		{
			In: `[{"method": "foo", "id": 1}, {"method": "bar", "id": 2}]`,
			Want: []Message{
				{ID: json.RawMessage("1"), Request: &Request{Method: "foo"}},
				{ID: json.RawMessage("2"), Request: &Request{Method: "bar"}},
			},
		},
	}

	for i, tc := range tests {
		buf := bytes.NewBufferString(tc.In)
		codec := IOCodec(nopCloser{buf})
		got := make([]Message, 0, len(tc.Want))
		for j := 0; j < len(tc.Want); j++ {
			if msg, err := codec.ReadMessage(); err != nil {
				t.Fatal(err)
			} else {
				got = append(got, *msg)
			}
		}
		sort.Sort(BatchByID(got))
		if !marshalEqual(got, tc.Want) {
			t.Errorf("[case #%d]\n   got: %v;\n  want: %v", i, got, tc.Want)
		}
	}
}
