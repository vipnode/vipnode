package jsonrpc2

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestReply(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	codec1 := jsonCodec{
		encoder: json.NewEncoder(&buf2),
		decoder: json.NewDecoder(&buf1),
	}

	codec2 := jsonCodec{
		encoder: json.NewEncoder(&buf1),
		decoder: json.NewDecoder(&buf2),
	}

	sentMsg := &Message{
		ID:      json.RawMessage("1"),
		Request: &Request{Method: "foo"},
	}

	if err := codec1.WriteMessage(sentMsg); err != nil {
		t.Fatal(err)
	}

	gotMsg, err := codec2.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	assertEqualJSON(t, gotMsg, sentMsg, "Write/Read message fail")

	sentResp := &Response{Result: json.RawMessage("42")}
	if err := gotMsg.Request.Reply(sentResp); err != nil {
		t.Fatal(err)
	}

	expectMsg := &Message{
		ID:       sentMsg.ID,
		Version:  Version,
		Response: sentResp,
	}
	gotMsg, err = codec1.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	assertEqualJSON(t, gotMsg, expectMsg, "Reply/Read message fail")

	sentBatch := []Message{
		Message{
			ID:      json.RawMessage("2"),
			Request: &Request{Method: "bar"},
		},
		Message{
			// No ID, should be treated as a notification (ignored)
			Request: &Request{Method: "baz"},
		},
		Message{
			ID:      json.RawMessage("3"),
			Request: &Request{Method: "quux"},
		},
	}
	if err = codec1.WriteBatch(sentBatch); err != nil {
		t.Fatal(err)
	}

	// This message is sent stand-alone, not as part of the batch.
	if err := codec1.WriteMessage(&Message{
		ID:      json.RawMessage("4"),
		Request: &Request{Method: "stop"},
	}); err != nil {
		t.Fatal(err)
	}

	// Assert that the wire format was batched
	got, want := buf2.String(), `[{"method":"bar","id":2},{"method":"baz"},{"method":"quux","id":3}]
{"method":"stop","id":4}
`
	if got != want {
		t.Errorf("incorrect wire payload after batched write:\n   got: %s\n  want: %s", got, want)
	}
	buf2.WriteString(got) // Put it back

	replied := 0
	for i := 0; i < 4; i++ {
		msg, err := codec2.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if err := msg.Reply(&Response{
			Result: json.RawMessage("42"),
		}); err == nil {
			replied += 1
		} else if msg.ID == nil && err == ErrReplyNotAvailable {
			// Expected for notification
		} else if err != nil {
			t.Fatal(err)
		}
	}

	if got, want := replied, 3; got != want {
		t.Errorf("wrong number of successful replies: got %d; want %d", got, want)
	}

	// Assert that the wire format was batched
	// FIXME: This test is unnecessarily order-sensitive, fix it when it inevitably breaks
	got, want = buf1.String(), `[{"result":42,"id":3,"jsonrpc":"2.0"},{"result":42,"id":2,"jsonrpc":"2.0"}]
{"result":42,"id":4,"jsonrpc":"2.0"}
`
	if got != want {
		t.Errorf("incorrect wire payload after batched reply:\n   got: %s\n  want: %s", got, want)
	}
	buf1.WriteString(got) // Put it back

	var receivedIDs []string
	for i := 0; i < replied; i++ {
		msg, err := codec1.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Compare(msg.Response.Result, []byte("42")) != 0 {
			t.Errorf("wrong result on batched response: %v", msg)
		}
		receivedIDs = append(receivedIDs, string(msg.ID))
	}
	sort.Strings(receivedIDs)
	if got, want := receivedIDs, []string{"2", "3", "4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("missing received IDs: got %s; want %s", got, want)
	}
}
