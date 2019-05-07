package jsonrpc2

import "testing"

func TestMessageFormat(t *testing.T) {
	msg := &Message{
		ID:      []byte("42"),
		Version: "2.0",
	}

	got, want := msg.String(), `{"id":42,"jsonrpc":"2.0"}`
	if got != want {
		t.Errorf("wrong message string formatting:\n  got: %s;\n want: %s", got, want)
	}
}
