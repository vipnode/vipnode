package jsonrpc2

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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
		In    string
		Want  []Message
		Error bool
	}{
		{
			In: `[{"method": "foo", "id": 1}]`,
			Want: []Message{
				{ID: json.RawMessage("1"), Request: &Request{Method: "foo"}},
			},
		},
		{
			In: `
				[
					{"method": "foo", "id": 2},
					{"method": "bar", "id": 3}
				]
			`,
			Want: []Message{
				{ID: json.RawMessage("2"), Request: &Request{Method: "foo"}},
				{ID: json.RawMessage("3"), Request: &Request{Method: "bar"}},
			},
		},
		{
			In:    `[]`,
			Error: true,
		},
		{
			In: `
				[
					{"method": "foo", "id": 4}, {"method": "bar", "id": 5}
				]
				{"method": "baaz", "id": 6}
				[
					{"method": "quux", "id": 7}
				]
			`,
			Want: []Message{
				{ID: json.RawMessage("4"), Request: &Request{Method: "foo"}},
				{ID: json.RawMessage("5"), Request: &Request{Method: "bar"}},
				{ID: json.RawMessage("6"), Request: &Request{Method: "baaz"}},
				{ID: json.RawMessage("7"), Request: &Request{Method: "quux"}},
			},
		},
	}

	logger.SetOutput(os.Stderr)

	for i, tc := range tests {
		buf := bytes.NewBufferString(tc.In)
		codec := IOCodec(nopCloser{buf})
		got := make([]Message, 0, len(tc.Want))
		var err error
		var msg *Message
		num := len(tc.Want)
		if num == 0 && tc.Error {
			num = 1
		}
		for j := 0; j < num; j++ {
			msg, err = codec.ReadMessage()
			if err != nil {
				break
			} else {
				got = append(got, *msg)
			}
		}

		if tc.Error && err == nil {
			t.Errorf("[case #%d] missing error", i)
			continue
		}

		if !tc.Error && err != nil {
			t.Errorf("[case #%d] unexpected error: %v; In: %s", i, err, tc.In)
			continue
		}

		if tc.Error && err != nil {
			// Undefined behaviour
			continue
		}

		sort.Sort(BatchByID(got))
		assertEqualJSON(t, got, tc.Want, "[case #%d]", i)
	}
}
