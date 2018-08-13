package jsonrpc2

import (
	"bytes"
	"reflect"
	"testing"
)

func TestCodec(t *testing.T) {
	var buf bytes.Buffer

	codec := IOCodec(&buf, &buf)
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
