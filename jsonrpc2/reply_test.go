package jsonrpc2

import (
	"bytes"
	"encoding/json"
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
		Response: sentResp,
	}
	gotMsg, err = codec1.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	assertEqualJSON(t, gotMsg, expectMsg, "Reply/Read message fail")
}
